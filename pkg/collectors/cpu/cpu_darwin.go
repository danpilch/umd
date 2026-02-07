//go:build darwin

package cpu

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/danpilch/umd/pkg/use"
)

/*
#include <mach/mach.h>
#include <mach/processor_info.h>
#include <mach/mach_host.h>
*/
import "C"

// CPUTicks holds CPU tick counts.
type CPUTicks struct {
	User   uint64
	System uint64
	Idle   uint64
	Nice   uint64
}

// Total returns total ticks.
func (t CPUTicks) Total() uint64 {
	return t.User + t.System + t.Idle + t.Nice
}

// Busy returns busy ticks.
func (t CPUTicks) Busy() uint64 {
	return t.User + t.System + t.Nice
}

// Collect gathers CPU USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Utilization
	util, err := c.getUtilization()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Utilization,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "host_processor_info",
		})
	} else {
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.1f%%", util),
			RawValue:    util,
			Status:      thresholds.EvaluateUtilization(util),
			Description: "CPU busy percentage",
			Command:     "host_processor_info",
		})
	}

	// Saturation (load average)
	sat, load, err := c.getSaturation()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Saturation,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "sysctl vm.loadavg",
		})
	} else {
		status := use.StatusOK
		if sat > 1.0 {
			status = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%.2f", load),
			RawValue:    sat,
			Status:      status,
			Description: fmt.Sprintf("Load average (1min) / CPU count (%d)", runtime.NumCPU()),
			Command:     "sysctl vm.loadavg",
		})
	}

	// Errors (from system.log - best effort)
	errCount := c.getErrors()
	checks = append(checks, use.Check{
		Resource:    "CPU",
		Type:        use.Errors,
		Value:       fmt.Sprintf("%d", errCount),
		RawValue:    float64(errCount),
		Status:      use.EvaluateErrors(errCount),
		Description: "CPU errors from system log",
		Command:     "log show",
	})

	return checks, nil
}

// getUtilization calculates CPU utilization using Mach APIs.
func (c *Collector) getUtilization() (float64, error) {
	ticks1, err := getCPUTicks()
	if err != nil {
		return 0, err
	}

	time.Sleep(100 * time.Millisecond)

	ticks2, err := getCPUTicks()
	if err != nil {
		return 0, err
	}

	totalDelta := float64(ticks2.Total() - ticks1.Total())
	if totalDelta == 0 {
		return 0, nil
	}

	busyDelta := float64(ticks2.Busy() - ticks1.Busy())
	return (busyDelta / totalDelta) * 100, nil
}

// getCPUTicks retrieves CPU tick counts using Mach host_processor_info.
func getCPUTicks() (CPUTicks, error) {
	var (
		numCPU         C.natural_t
		cpuInfo        *C.integer_t
		numCPUInfo     C.mach_msg_type_number_t
	)

	host := C.mach_host_self()
	ret := C.host_processor_info(host, C.PROCESSOR_CPU_LOAD_INFO, &numCPU, (*C.processor_info_array_t)(unsafe.Pointer(&cpuInfo)), &numCPUInfo)
	if ret != C.KERN_SUCCESS {
		return CPUTicks{}, fmt.Errorf("host_processor_info failed: %d", ret)
	}
	defer C.vm_deallocate(C.mach_task_self_, C.vm_address_t(uintptr(unsafe.Pointer(cpuInfo))), C.vm_size_t(numCPUInfo)*C.vm_size_t(unsafe.Sizeof(C.integer_t(0))))

	var ticks CPUTicks
	cpuLoadInfo := (*[1 << 20]C.integer_t)(unsafe.Pointer(cpuInfo))

	for i := C.natural_t(0); i < numCPU; i++ {
		offset := i * C.CPU_STATE_MAX
		ticks.User += uint64(cpuLoadInfo[offset+C.CPU_STATE_USER])
		ticks.System += uint64(cpuLoadInfo[offset+C.CPU_STATE_SYSTEM])
		ticks.Idle += uint64(cpuLoadInfo[offset+C.CPU_STATE_IDLE])
		ticks.Nice += uint64(cpuLoadInfo[offset+C.CPU_STATE_NICE])
	}

	return ticks, nil
}

// getSaturation returns load average relative to CPU count.
func (c *Collector) getSaturation() (float64, float64, error) {
	cmd := exec.Command("sysctl", "-n", "vm.loadavg")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	// Output format: { 1.23 4.56 7.89 }
	str := strings.Trim(string(out), "{ }\n")
	fields := strings.Fields(str)
	if len(fields) < 1 {
		return 0, 0, fmt.Errorf("unexpected sysctl output format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, err
	}

	cpuCount := float64(runtime.NumCPU())
	return load1 / cpuCount, load1, nil
}

// getErrors checks for CPU-related errors in system logs.
func (c *Collector) getErrors() int64 {
	// Best effort - check system.log for CPU errors
	cmd := exec.Command("log", "show", "--predicate", "eventMessage contains 'CPU' AND eventMessage contains 'error'", "--last", "1h", "--style", "compact")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0
	}

	lines := strings.Split(out.String(), "\n")
	count := int64(0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "Timestamp") {
			count++
		}
	}
	return count
}
