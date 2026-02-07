//go:build darwin

package disk

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers disk USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0)

	// Get disk I/O stats from iostat
	ioStats, err := getIOStats()
	if err == nil {
		for disk, stats := range ioStats {
			// Utilization (KB/sec - can't get % easily on macOS)
			totalKBs := stats["KB/t"] * (stats["tps"]) // KB/transfer * transfers/sec
			checks = append(checks, use.Check{
				Resource:    fmt.Sprintf("Disk (%s)", disk),
				Type:        use.Utilization,
				Value:       fmt.Sprintf("%.1f KB/s", totalKBs),
				RawValue:    totalKBs,
				Status:      use.StatusOK, // Can't determine % without max throughput
				Description: "I/O throughput",
				Command:     "iostat",
			})

			// Saturation - use transfers per second as a proxy
			// High tps with low KB/t might indicate many small random IOs
			tps := stats["tps"]
			satStatus := use.StatusOK
			if tps > 1000 { // Arbitrary high threshold
				satStatus = use.StatusWarning
			}
			checks = append(checks, use.Check{
				Resource:    fmt.Sprintf("Disk (%s)", disk),
				Type:        use.Saturation,
				Value:       fmt.Sprintf("%.1f tps", tps),
				RawValue:    tps,
				Status:      satStatus,
				Description: "Transfers per second",
				Command:     "iostat",
			})

			// Errors (limited on macOS - check system.log)
			errCount := getDiskErrors()
			checks = append(checks, use.Check{
				Resource:    fmt.Sprintf("Disk (%s)", disk),
				Type:        use.Errors,
				Value:       fmt.Sprintf("%d", errCount),
				RawValue:    float64(errCount),
				Status:      use.EvaluateErrors(errCount),
				Description: "Disk errors from system log",
				Command:     "log show",
			})
		}
	}

	// Add filesystem capacity checks
	mountPoints := getMainMountPoints()
	checks = append(checks, GetFilesystemChecks(thresholds, mountPoints)...)

	return checks, nil
}

// getIOStats parses iostat output for disk statistics.
// iostat -d -c 2 outputs:
//               disk0
//     KB/t  tps  MB/s
//    24.44  232  5.53   <- first sample (cumulative since boot)
//    12.19   21  0.25   <- second sample (current activity)
func getIOStats() (map[string]map[string]float64, error) {
	cmd := exec.Command("iostat", "-d", "-c", "2")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]map[string]float64)
	lines := strings.Split(string(out), "\n")

	if len(lines) < 4 {
		return stats, nil
	}

	// Parse disk names from first line (e.g., "              disk0")
	diskLine := strings.TrimSpace(lines[0])
	diskNames := strings.Fields(diskLine)

	// Skip header line (KB/t tps MB/s)
	// The last data line is the second sample (current activity)
	var lastDataLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.Contains(line, "KB/t") && !strings.Contains(line, "disk") {
			lastDataLine = line
			break
		}
	}

	if lastDataLine == "" {
		return stats, nil
	}

	// Parse the data - each disk has 3 values (KB/t, tps, MB/s)
	fields := strings.Fields(lastDataLine)
	for i, diskName := range diskNames {
		offset := i * 3
		if offset+2 >= len(fields) {
			continue
		}

		s := make(map[string]float64)
		s["KB/t"], _ = strconv.ParseFloat(fields[offset], 64)
		s["tps"], _ = strconv.ParseFloat(fields[offset+1], 64)
		s["MB/s"], _ = strconv.ParseFloat(fields[offset+2], 64)

		stats[diskName] = s
	}

	return stats, nil
}

// getDiskErrors checks for disk-related errors in system logs.
func getDiskErrors() int64 {
	cmd := exec.Command("log", "show", "--predicate", "(subsystem == 'com.apple.iokit.IOStorageFamily') AND (eventMessage contains 'error')", "--last", "1h", "--style", "compact")
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

// getMainMountPoints returns the main mount points to check.
func getMainMountPoints() []string {
	// Always check root
	points := []string{"/"}

	// Get mount points from df
	cmd := exec.Command("df", "-P")
	out, err := cmd.Output()
	if err != nil {
		return points
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	lineNum := 0
	seen := map[string]bool{"/": true}

	for scanner.Scan() {
		lineNum++
		// Skip header
		if lineNum == 1 {
			continue
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}

		device := fields[0]
		mp := fields[5]

		// Skip virtual filesystems and system volumes
		if strings.HasPrefix(device, "devfs") ||
			strings.HasPrefix(device, "map ") ||
			device == "none" ||
			strings.HasPrefix(mp, "/System/Volumes/") {
			continue
		}

		// Skip if already seen
		if seen[mp] {
			continue
		}

		seen[mp] = true
		points = append(points, mp)
	}

	return points
}
