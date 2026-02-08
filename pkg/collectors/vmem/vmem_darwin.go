//go:build darwin

package vmem

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers virtual memory USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	stats, err := readVMStat()
	if err != nil {
		return nil, err
	}

	// Utilization: page faults (major)
	pageFaults := stats["Page faults"]
	status := use.StatusOK
	checks = append(checks, use.Check{
		Resource:    "VMem",
		Type:        use.Utilization,
		Value:       fmt.Sprintf("%d faults (total)", pageFaults),
		RawValue:    float64(pageFaults),
		Status:      status,
		Description: "Page faults (cumulative from vm_stat)",
		Command:     "vm_stat",
	})

	// Saturation: pageins + pageouts
	pageins := stats["Pageins"]
	pageouts := stats["Pageouts"]
	satTotal := pageins + pageouts
	satStatus := use.StatusOK
	if pageouts > 0 {
		satStatus = use.StatusWarning
	}
	checks = append(checks, use.Check{
		Resource:    "VMem",
		Type:        use.Saturation,
		Value:       fmt.Sprintf("%d pageins, %d pageouts", pageins, pageouts),
		RawValue:    float64(satTotal),
		Status:      satStatus,
		Description: "Page ins/outs indicate swap activity",
		Command:     "vm_stat",
	})

	// Errors: swapouts as a pressure indicator
	swapouts := stats["Swapouts"]
	errStatus := use.StatusOK
	if swapouts > 0 {
		errStatus = use.StatusWarning
	}
	checks = append(checks, use.Check{
		Resource:    "VMem",
		Type:        use.Errors,
		Value:       fmt.Sprintf("%d swapouts", swapouts),
		RawValue:    float64(swapouts),
		Status:      errStatus,
		Description: "Swap outs indicate severe memory pressure",
		Command:     "vm_stat",
	})

	return checks, nil
}

func readVMStat() (map[string]uint64, error) {
	cmd := exec.Command("vm_stat")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]uint64)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Mach Virtual Memory") {
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
	return stats, nil
}
