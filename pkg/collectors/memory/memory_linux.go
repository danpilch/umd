//go:build linux

package memory

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers memory USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	memInfo, err := readMemInfo()
	if err != nil {
		return nil, err
	}

	// Utilization
	util := c.calculateUtilization(memInfo)
	checks = append(checks, use.Check{
		Resource:    "Memory",
		Type:        use.Utilization,
		Value:       fmt.Sprintf("%.1f%%", util),
		RawValue:    util,
		Status:      thresholds.EvaluateUtilization(util),
		Description: "Memory used percentage",
		Command:     "/proc/meminfo",
	})

	// Saturation (swap usage)
	sat, satDesc := c.calculateSaturation(memInfo)
	satStatus := use.StatusOK
	if sat > 0 {
		satStatus = use.StatusWarning
	}
	checks = append(checks, use.Check{
		Resource:    "Memory",
		Type:        use.Saturation,
		Value:       satDesc,
		RawValue:    sat,
		Status:      satStatus,
		Description: "Swap usage indicates memory pressure",
		Command:     "/proc/meminfo",
	})

	// Errors (OOM killer)
	errCount := c.getErrors()
	checks = append(checks, use.Check{
		Resource:    "Memory",
		Type:        use.Errors,
		Value:       fmt.Sprintf("%d", errCount),
		RawValue:    float64(errCount),
		Status:      use.EvaluateErrors(errCount),
		Description: "OOM killer invocations",
		Command:     "dmesg",
	})

	return checks, nil
}

// readMemInfo parses /proc/meminfo into a map.
func readMemInfo() (map[string]uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		info[key] = value
	}

	return info, scanner.Err()
}

// calculateUtilization computes memory utilization percentage.
func (c *Collector) calculateUtilization(info map[string]uint64) float64 {
	total := info["MemTotal"]
	if total == 0 {
		return 0
	}

	// Available memory (Linux 3.14+) or fallback to Free + Buffers + Cached
	available, ok := info["MemAvailable"]
	if !ok {
		available = info["MemFree"] + info["Buffers"] + info["Cached"]
	}

	used := total - available
	return (float64(used) / float64(total)) * 100
}

// calculateSaturation computes memory saturation based on swap usage.
func (c *Collector) calculateSaturation(info map[string]uint64) (float64, string) {
	swapTotal := info["SwapTotal"]
	swapFree := info["SwapFree"]

	if swapTotal == 0 {
		return 0, "0 (no swap)"
	}

	swapUsed := swapTotal - swapFree
	swapPercent := (float64(swapUsed) / float64(swapTotal)) * 100

	return swapPercent, fmt.Sprintf("%.1f%% swap", swapPercent)
}

// getErrors checks for OOM killer invocations.
func (c *Collector) getErrors() int64 {
	// Best effort - try to read dmesg or /var/log/kern.log
	file, err := os.Open("/var/log/kern.log")
	if err != nil {
		return 0
	}
	defer file.Close()

	var count int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, "oom") || strings.Contains(line, "out of memory") {
			count++
		}
	}

	return count
}
