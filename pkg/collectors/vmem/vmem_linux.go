//go:build linux

package vmem

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers virtual memory USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Read vmstat twice for rate calculations
	vmstat1, err := readVMStat()
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	vmstat2, err := readVMStat()
	if err != nil {
		return nil, err
	}

	// Utilization: major page fault rate
	pgmajfault1 := vmstat1["pgmajfault"]
	pgmajfault2 := vmstat2["pgmajfault"]
	faultRate := float64(pgmajfault2-pgmajfault1) * 10 // scale to per-second

	status := use.StatusOK
	if faultRate > 10 {
		status = use.StatusWarning
	}
	if faultRate > 100 {
		status = use.StatusError
	}
	checks = append(checks, use.Check{
		Resource:    "VMem",
		Type:        use.Utilization,
		Value:       fmt.Sprintf("%.1f faults/s", faultRate),
		RawValue:    faultRate,
		Status:      status,
		Description: "Major page fault rate (pgmajfault)",
		Command:     "/proc/vmstat",
	})

	// Saturation: swap I/O rate + page scan rate
	pswpin1 := vmstat1["pswpin"]
	pswpout1 := vmstat1["pswpout"]
	pswpin2 := vmstat2["pswpin"]
	pswpout2 := vmstat2["pswpout"]
	swapRate := float64((pswpin2-pswpin1)+(pswpout2-pswpout1)) * 10

	pgscanKswapd1 := vmstat1["pgscan_kswapd"]
	pgscanDirect1 := vmstat1["pgscan_direct"]
	pgscanKswapd2 := vmstat2["pgscan_kswapd"]
	pgscanDirect2 := vmstat2["pgscan_direct"]
	scanRate := float64((pgscanKswapd2-pgscanKswapd1)+(pgscanDirect2-pgscanDirect1)) * 10

	satStatus := use.StatusOK
	if swapRate > 0 || scanRate > 0 {
		satStatus = use.StatusWarning
	}
	checks = append(checks, use.Check{
		Resource:    "VMem",
		Type:        use.Saturation,
		Value:       fmt.Sprintf("swap: %.0f/s, scan: %.0f/s", swapRate, scanRate),
		RawValue:    swapRate + scanRate,
		Status:      satStatus,
		Description: "Swap I/O rate + page scan rate",
		Command:     "/proc/vmstat",
	})

	// Errors: dirty page ratio from /proc/meminfo
	dirtyRatio, err := getDirtyRatio()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "VMem",
			Type:        use.Errors,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/meminfo",
		})
	} else {
		errStatus := use.StatusOK
		if dirtyRatio > 10 {
			errStatus = use.StatusWarning
		}
		if dirtyRatio > 30 {
			errStatus = use.StatusError
		}
		checks = append(checks, use.Check{
			Resource:    "VMem",
			Type:        use.Errors,
			Value:       fmt.Sprintf("%.1f%% dirty", dirtyRatio),
			RawValue:    dirtyRatio,
			Status:      errStatus,
			Description: "Dirty page ratio (Dirty/MemTotal)",
			Command:     "/proc/meminfo",
		})
	}

	return checks, nil
}

func readVMStat() (map[string]uint64, error) {
	file, err := os.Open("/proc/vmstat")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 {
			val, _ := strconv.ParseUint(fields[1], 10, 64)
			stats[fields[0]] = val
		}
	}
	return stats, scanner.Err()
}

func getDirtyRatio() (float64, error) {
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
	dirty := info["Dirty"]
	if total == 0 {
		return 0, fmt.Errorf("MemTotal is 0")
	}
	return (float64(dirty) / float64(total)) * 100, nil
}
