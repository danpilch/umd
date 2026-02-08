//go:build linux

package scheduler

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

// Collect gathers scheduler USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Utilization: run queue depth from /proc/stat procs_running
	runQueue, err := getRunQueueDepth()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Utilization,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/stat",
		})
	} else {
		cpuCount := runtime.NumCPU()
		status := use.StatusOK
		if runQueue > int64(cpuCount*2) {
			status = use.StatusWarning
		}
		if runQueue > int64(cpuCount*4) {
			status = use.StatusError
		}
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%d procs (CPUs: %d)", runQueue, cpuCount),
			RawValue:    float64(runQueue),
			Status:      status,
			Description: "Run queue depth (procs_running)",
			Command:     "/proc/stat",
		})
	}

	// Saturation: context switches per second
	csw, err := getContextSwitchRate()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Saturation,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/stat",
		})
	} else {
		status := use.StatusOK
		// High context switch rates indicate scheduler pressure
		if csw > 100000 {
			status = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%.0f csw/s", csw),
			RawValue:    csw,
			Status:      status,
			Description: "Context switches per second",
			Command:     "/proc/stat",
		})
	}

	// Errors: involuntary context switch ratio from /proc/self/status
	involCSW, err := getInvoluntaryCSW()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Errors,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/self/status",
		})
	} else {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Errors,
			Value:       fmt.Sprintf("%d", involCSW),
			RawValue:    float64(involCSW),
			Status:      use.EvaluateErrors(involCSW),
			Description: "Involuntary context switches (self)",
			Command:     "/proc/self/status",
		})
	}

	return checks, nil
}

func getRunQueueDepth() (int64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "procs_running") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected procs_running format")
			}
			return strconv.ParseInt(fields[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("procs_running not found in /proc/stat")
}

func getContextSwitchRate() (float64, error) {
	csw1, err := readCtxtFromStat()
	if err != nil {
		return 0, err
	}

	time.Sleep(100 * time.Millisecond)

	csw2, err := readCtxtFromStat()
	if err != nil {
		return 0, err
	}

	// Scale to per-second (100ms sample * 10)
	return float64(csw2-csw1) * 10, nil
}

func readCtxtFromStat() (uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ctxt ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected ctxt format")
			}
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("ctxt not found in /proc/stat")
}

func getInvoluntaryCSW() (int64, error) {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "nonvoluntary_ctxt_switches:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected format")
			}
			return strconv.ParseInt(fields[1], 10, 64)
		}
	}
	return 0, nil
}
