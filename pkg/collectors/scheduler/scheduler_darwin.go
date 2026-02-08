//go:build darwin

package scheduler

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers scheduler USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Utilization: approximate run queue from load average
	load, err := getLoadAverage()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Utilization,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "sysctl vm.loadavg",
		})
	} else {
		cpuCount := runtime.NumCPU()
		status := use.StatusOK
		if load > float64(cpuCount*2) {
			status = use.StatusWarning
		}
		if load > float64(cpuCount*4) {
			status = use.StatusError
		}
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.2f load (CPUs: %d)", load, cpuCount),
			RawValue:    load,
			Status:      status,
			Description: "1-min load average as run queue proxy",
			Command:     "sysctl vm.loadavg",
		})
	}

	// Saturation: context switches from host_statistics
	csw, err := getContextSwitches()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Saturation,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "sysctl",
		})
	} else {
		checks = append(checks, use.Check{
			Resource:    "Scheduler",
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%d csw (total)", csw),
			RawValue:    float64(csw),
			Status:      use.StatusOK,
			Description: "Context switches (cumulative)",
			Command:     "sysctl",
		})
	}

	// Errors: not directly available on macOS, report 0
	checks = append(checks, use.Check{
		Resource:    "Scheduler",
		Type:        use.Errors,
		Value:       "0",
		RawValue:    0,
		Status:      use.StatusOK,
		Description: "No scheduler error metrics available on macOS",
		Command:     "n/a",
	})

	return checks, nil
}

func getLoadAverage() (float64, error) {
	cmd := exec.Command("sysctl", "-n", "vm.loadavg")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	str := strings.Trim(string(out), "{ }\n")
	fields := strings.Fields(str)
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected sysctl output")
	}
	return strconv.ParseFloat(fields[0], 64)
}

func getContextSwitches() (int64, error) {
	cmd := exec.Command("sysctl", "-n", "vm.stats.sys.v_swtch")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}
