//go:build linux

package cpu

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/danpilch/umd/pkg/use"
)

// CPUStats holds raw CPU statistics from /proc/stat.
type CPUStats struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

// Total returns the total CPU time.
func (s CPUStats) Total() uint64 {
	return s.User + s.Nice + s.System + s.Idle + s.IOWait + s.IRQ + s.SoftIRQ + s.Steal
}

// Busy returns the busy CPU time (non-idle).
func (s CPUStats) Busy() uint64 {
	return s.User + s.Nice + s.System + s.IRQ + s.SoftIRQ + s.Steal
}

// Collect gathers CPU USE metrics on Linux.
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
			Command:     "/proc/stat",
		})
	} else {
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.1f%%", util),
			RawValue:    util,
			Status:      thresholds.EvaluateUtilization(util),
			Description: "CPU busy percentage",
			Command:     "/proc/stat",
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
			Command:     "/proc/loadavg",
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
			Command:     "/proc/loadavg",
		})
	}

	// Errors (from dmesg - best effort)
	errCount, err := c.getErrors()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Errors,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "dmesg",
		})
	} else {
		checks = append(checks, use.Check{
			Resource:    "CPU",
			Type:        use.Errors,
			Value:       fmt.Sprintf("%d", errCount),
			RawValue:    float64(errCount),
			Status:      use.EvaluateErrors(errCount),
			Description: "CPU errors from kernel log",
			Command:     "dmesg",
		})
	}

	return checks, nil
}

// getUtilization calculates CPU utilization by sampling /proc/stat twice.
func (c *Collector) getUtilization() (float64, error) {
	stats1, err := readCPUStats()
	if err != nil {
		return 0, err
	}

	time.Sleep(100 * time.Millisecond)

	stats2, err := readCPUStats()
	if err != nil {
		return 0, err
	}

	totalDelta := float64(stats2.Total() - stats1.Total())
	if totalDelta == 0 {
		return 0, nil
	}

	busyDelta := float64(stats2.Busy() - stats1.Busy())
	return (busyDelta / totalDelta) * 100, nil
}

// readCPUStats reads CPU statistics from /proc/stat.
func readCPUStats() (CPUStats, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return CPUStats{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return CPUStats{}, fmt.Errorf("unexpected /proc/stat format")
			}

			stats := CPUStats{}
			stats.User, _ = strconv.ParseUint(fields[1], 10, 64)
			stats.Nice, _ = strconv.ParseUint(fields[2], 10, 64)
			stats.System, _ = strconv.ParseUint(fields[3], 10, 64)
			stats.Idle, _ = strconv.ParseUint(fields[4], 10, 64)
			stats.IOWait, _ = strconv.ParseUint(fields[5], 10, 64)
			stats.IRQ, _ = strconv.ParseUint(fields[6], 10, 64)
			stats.SoftIRQ, _ = strconv.ParseUint(fields[7], 10, 64)
			if len(fields) > 8 {
				stats.Steal, _ = strconv.ParseUint(fields[8], 10, 64)
			}
			return stats, nil
		}
	}

	return CPUStats{}, fmt.Errorf("cpu line not found in /proc/stat")
}

// getSaturation returns load average relative to CPU count.
func (c *Collector) getSaturation() (float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, err
	}

	cpuCount := float64(runtime.NumCPU())
	return load1 / cpuCount, load1, nil
}

// getErrors checks for CPU-related errors in kernel logs.
func (c *Collector) getErrors() (int64, error) {
	// Try to read from /var/log/kern.log or dmesg
	// This is best-effort and may require elevated privileges
	file, err := os.Open("/var/log/kern.log")
	if err != nil {
		// Return 0 errors if we can't read the log
		return 0, nil
	}
	defer file.Close()

	var count int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, "mce") || // Machine Check Exception
			strings.Contains(line, "cpu error") ||
			strings.Contains(line, "hardware error") {
			count++
		}
	}

	return count, nil
}
