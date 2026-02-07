//go:build darwin

package memory

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"

	"github.com/danpilch/umd/pkg/use"
)

/*
#include <stdlib.h>
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <sys/sysctl.h>
*/
import "C"

// Collect gathers memory USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Utilization
	util, err := c.getUtilization()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Memory",
			Type:        use.Utilization,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "host_statistics64",
		})
	} else {
		checks = append(checks, use.Check{
			Resource:    "Memory",
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.1f%%", util),
			RawValue:    util,
			Status:      thresholds.EvaluateUtilization(util),
			Description: "Memory used percentage",
			Command:     "host_statistics64",
		})
	}

	// Saturation (vm_stat pageouts)
	sat, satDesc, err := c.getSaturation()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Memory",
			Type:        use.Saturation,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "vm_stat",
		})
	} else {
		status := use.StatusOK
		if sat > 0 {
			status = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    "Memory",
			Type:        use.Saturation,
			Value:       satDesc,
			RawValue:    sat,
			Status:      status,
			Description: "Page outs indicate memory pressure",
			Command:     "vm_stat",
		})
	}

	// Errors (from system.log - best effort)
	errCount := c.getErrors()
	checks = append(checks, use.Check{
		Resource:    "Memory",
		Type:        use.Errors,
		Value:       fmt.Sprintf("%d", errCount),
		RawValue:    float64(errCount),
		Status:      use.EvaluateErrors(errCount),
		Description: "Memory errors from system log",
		Command:     "log show",
	})

	return checks, nil
}

// getUtilization calculates memory utilization using Mach APIs.
func (c *Collector) getUtilization() (float64, error) {
	// Get total physical memory
	var totalMem C.uint64_t
	size := C.size_t(unsafe.Sizeof(totalMem))
	name := C.CString("hw.memsize")
	defer C.free(unsafe.Pointer(name))

	if C.sysctlbyname(name, unsafe.Pointer(&totalMem), &size, nil, 0) != 0 {
		return 0, fmt.Errorf("failed to get hw.memsize")
	}

	// Get memory statistics
	var vmStats C.vm_statistics64_data_t
	count := C.mach_msg_type_number_t(C.HOST_VM_INFO64_COUNT)

	host := C.mach_host_self()
	ret := C.host_statistics64(host, C.HOST_VM_INFO64, (*C.integer_t)(unsafe.Pointer(&vmStats)), &count)
	if ret != C.KERN_SUCCESS {
		return 0, fmt.Errorf("host_statistics64 failed: %d", ret)
	}

	pageSize := uint64(C.vm_kernel_page_size)

	// Calculate used memory
	// Free = free_count * page_size
	// Used = total - free - (inactive + purgeable + speculative that can be reclaimed)
	freePages := uint64(vmStats.free_count)
	inactivePages := uint64(vmStats.inactive_count)
	purgeablePages := uint64(vmStats.purgeable_count)
	speculativePages := uint64(vmStats.speculative_count)

	freeMem := (freePages + inactivePages + purgeablePages + speculativePages) * pageSize
	usedMem := uint64(totalMem) - freeMem

	util := (float64(usedMem) / float64(totalMem)) * 100
	return util, nil
}

// getSaturation checks for pageouts indicating memory pressure.
func (c *Collector) getSaturation() (float64, string, error) {
	cmd := exec.Command("vm_stat")
	out, err := cmd.Output()
	if err != nil {
		return 0, "", err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	var pageouts uint64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Pageouts:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pageouts, _ = strconv.ParseUint(strings.TrimSuffix(fields[1], "."), 10, 64)
			}
		}
	}

	// Pageouts > 0 indicates memory pressure has occurred
	return float64(pageouts), fmt.Sprintf("%d pageouts", pageouts), nil
}

// getErrors checks for memory-related errors in system logs.
func (c *Collector) getErrors() int64 {
	// Best effort - check for memory pressure and jetsam events
	cmd := exec.Command("log", "show", "--predicate", "(eventMessage contains 'jetsam') OR (eventMessage contains 'memory pressure')", "--last", "1h", "--style", "compact")
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
