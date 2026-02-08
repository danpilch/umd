//go:build darwin

package crosscheck

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"
)

/*
#include <stdlib.h>
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <mach/processor_info.h>
#include <sys/sysctl.h>
*/
import "C"

// GetCPUSources returns CPU utilization from multiple macOS sources.
func GetCPUSources() []Source {
	var sources []Source

	// Source 1: Mach host_processor_info
	if util, err := machCPUUtilization(); err == nil {
		sources = append(sources, Source{
			Name:  "host_processor_info",
			Value: util,
			Unit:  "%",
		})
	}

	// Source 2: top -l1
	if util, err := topCPUUtilization(); err == nil {
		sources = append(sources, Source{
			Name:  "top",
			Value: util,
			Unit:  "%",
		})
	}

	return sources
}

// GetMemorySources returns memory utilization from multiple macOS sources.
func GetMemorySources() []Source {
	var sources []Source

	// Source 1: Mach host_statistics64
	if util, err := machMemoryUtilization(); err == nil {
		sources = append(sources, Source{
			Name:  "host_statistics64",
			Value: util,
			Unit:  "%",
		})
	}

	// Source 2: vm_stat + sysctl hw.memsize
	if util, err := vmstatMemoryUtilization(); err == nil {
		sources = append(sources, Source{
			Name:  "vm_stat+sysctl",
			Value: util,
			Unit:  "%",
		})
	}

	return sources
}

func machCPUUtilization() (float64, error) {
	var (
		numCPU     C.natural_t
		cpuInfo    *C.integer_t
		numCPUInfo C.mach_msg_type_number_t
	)

	host := C.mach_host_self()
	ret := C.host_processor_info(host, C.PROCESSOR_CPU_LOAD_INFO, &numCPU,
		(*C.processor_info_array_t)(unsafe.Pointer(&cpuInfo)), &numCPUInfo)
	if ret != C.KERN_SUCCESS {
		return 0, nil
	}
	defer C.vm_deallocate(C.mach_task_self_,
		C.vm_address_t(uintptr(unsafe.Pointer(cpuInfo))),
		C.vm_size_t(numCPUInfo)*C.vm_size_t(unsafe.Sizeof(C.integer_t(0))))

	cpuLoadInfo := (*[1 << 20]C.integer_t)(unsafe.Pointer(cpuInfo))
	var user, system, idle, nice uint64
	for i := C.natural_t(0); i < numCPU; i++ {
		offset := i * C.CPU_STATE_MAX
		user += uint64(cpuLoadInfo[offset+C.CPU_STATE_USER])
		system += uint64(cpuLoadInfo[offset+C.CPU_STATE_SYSTEM])
		idle += uint64(cpuLoadInfo[offset+C.CPU_STATE_IDLE])
		nice += uint64(cpuLoadInfo[offset+C.CPU_STATE_NICE])
	}

	total := float64(user + system + idle + nice)
	if total == 0 {
		return 0, nil
	}
	busy := float64(user + system + nice)
	return (busy / total) * 100, nil
}

func topCPUUtilization() (float64, error) {
	cmd := exec.Command("top", "-l", "1", "-n", "0", "-s", "0")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "CPU usage:") {
			// Format: "CPU usage: 5.55% user, 10.81% sys, 83.63% idle"
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "idle" && i > 0 {
					idleStr := strings.TrimSuffix(parts[i-1], "%")
					idle, err := strconv.ParseFloat(idleStr, 64)
					if err != nil {
						return 0, err
					}
					return 100 - idle, nil
				}
			}
		}
	}
	return 0, nil
}

func machMemoryUtilization() (float64, error) {
	var totalMem C.uint64_t
	size := C.size_t(unsafe.Sizeof(totalMem))
	name := C.CString("hw.memsize")
	defer C.free(unsafe.Pointer(name))

	if C.sysctlbyname(name, unsafe.Pointer(&totalMem), &size, nil, 0) != 0 {
		return 0, nil
	}

	var vmStats C.vm_statistics64_data_t
	count := C.mach_msg_type_number_t(C.HOST_VM_INFO64_COUNT)
	host := C.mach_host_self()
	ret := C.host_statistics64(host, C.HOST_VM_INFO64,
		(*C.integer_t)(unsafe.Pointer(&vmStats)), &count)
	if ret != C.KERN_SUCCESS {
		return 0, nil
	}

	pageSize := uint64(C.vm_kernel_page_size)
	freePages := uint64(vmStats.free_count)
	inactivePages := uint64(vmStats.inactive_count)
	purgeablePages := uint64(vmStats.purgeable_count)
	speculativePages := uint64(vmStats.speculative_count)

	freeMem := (freePages + inactivePages + purgeablePages + speculativePages) * pageSize
	usedMem := uint64(totalMem) - freeMem
	return (float64(usedMem) / float64(totalMem)) * 100, nil
}

func vmstatMemoryUtilization() (float64, error) {
	// Get total memory from sysctl
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	totalMem, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, err
	}

	// Get vm_stat output
	cmd = exec.Command("vm_stat")
	out, err = cmd.Output()
	if err != nil {
		return 0, err
	}

	// Parse page size and page counts
	var pageSize uint64 = 4096
	stats := make(map[string]uint64)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			// Extract page size if mentioned
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "size" && i > 0 {
					ps, err := strconv.ParseUint(parts[i+1], 10, 64)
					if err == nil {
						pageSize = ps
					}
				}
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(strings.TrimSuffix(parts[1], "."))
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		stats[key] = val
	}

	freePages := stats["Pages free"] + stats["Pages inactive"] +
		stats["Pages purgeable"] + stats["Pages speculative"]
	freeMem := freePages * pageSize
	usedMem := totalMem - freeMem
	return (float64(usedMem) / float64(totalMem)) * 100, nil
}
