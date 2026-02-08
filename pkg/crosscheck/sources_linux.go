//go:build linux

package crosscheck

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// GetCPUSources returns CPU utilization from multiple Linux sources.
func GetCPUSources() []Source {
	var sources []Source

	// Source 1: /proc/stat
	if util, err := procStatCPU(); err == nil {
		sources = append(sources, Source{
			Name:  "/proc/stat",
			Value: util,
			Unit:  "%",
		})
	}

	// Source 2: /proc/loadavg (instantaneous load / CPU count)
	if load, err := procLoadAvg(); err == nil {
		sources = append(sources, Source{
			Name:    "/proc/loadavg",
			Value:   load,
			Unit:    "load/cpu",
			RawData: "1-min load average normalized to CPU count",
		})
	}

	// Source 3: sysinfo syscall
	if util, err := sysinfoUptime(); err == nil {
		sources = append(sources, Source{
			Name:  "sysinfo",
			Value: util,
			Unit:  "load/cpu",
		})
	}

	return sources
}

// GetMemorySources returns memory utilization from multiple Linux sources.
func GetMemorySources() []Source {
	var sources []Source

	// Source 1: /proc/meminfo
	if util, err := procMeminfo(); err == nil {
		sources = append(sources, Source{
			Name:  "/proc/meminfo",
			Value: util,
			Unit:  "%",
		})
	}

	// Source 2: sysinfo syscall
	if util, err := sysinfoMemory(); err == nil {
		sources = append(sources, Source{
			Name:  "sysinfo",
			Value: util,
			Unit:  "%",
		})
	}

	return sources
}

func procStatCPU() (float64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return 0, fmt.Errorf("unexpected /proc/stat format")
			}
			user, _ := strconv.ParseUint(fields[1], 10, 64)
			nice, _ := strconv.ParseUint(fields[2], 10, 64)
			system, _ := strconv.ParseUint(fields[3], 10, 64)
			idle, _ := strconv.ParseUint(fields[4], 10, 64)
			iowait, _ := strconv.ParseUint(fields[5], 10, 64)
			irq, _ := strconv.ParseUint(fields[6], 10, 64)
			softirq, _ := strconv.ParseUint(fields[7], 10, 64)
			var steal uint64
			if len(fields) > 8 {
				steal, _ = strconv.ParseUint(fields[8], 10, 64)
			}

			total := float64(user + nice + system + idle + iowait + irq + softirq + steal)
			if total == 0 {
				return 0, nil
			}
			busy := float64(user + nice + system + irq + softirq + steal)
			return (busy / total) * 100, nil
		}
	}
	return 0, fmt.Errorf("cpu line not found")
}

func procLoadAvg() (float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected format")
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	cpus := getCPUCount()
	if cpus == 0 {
		cpus = 1
	}
	return load / float64(cpus) * 100, nil
}

func sysinfoUptime() (float64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	loads := info.Loads
	// Loads are scaled by 65536
	load1 := float64(loads[0]) / 65536.0
	cpus := getCPUCount()
	if cpus == 0 {
		cpus = 1
	}
	return load1 / float64(cpus) * 100, nil
}

func procMeminfo() (float64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	info := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		info[key] = val
	}

	total := info["MemTotal"]
	if total == 0 {
		return 0, fmt.Errorf("MemTotal is 0")
	}

	available, ok := info["MemAvailable"]
	if !ok {
		available = info["MemFree"] + info["Buffers"] + info["Cached"]
	}
	used := total - available
	return (float64(used) / float64(total)) * 100, nil
}

func sysinfoMemory() (float64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	unit := uint64(info.Unit)
	total := uint64(info.Totalram) * unit
	free := uint64(info.Freeram) * unit
	buffers := uint64(info.Bufferram) * unit

	if total == 0 {
		return 0, fmt.Errorf("total RAM is 0")
	}
	used := total - free - buffers
	return (float64(used) / float64(total)) * 100, nil
}

func getCPUCount() int {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 1
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

