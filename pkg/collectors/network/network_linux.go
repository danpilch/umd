//go:build linux

package network

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danpilch/umd/pkg/use"
)

// InterfaceStats holds network interface statistics.
type InterfaceStats struct {
	Name      string
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxErrors  uint64
	TxErrors  uint64
	RxDropped uint64
	TxDropped uint64
}

// Collect gathers network USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0)

	// Get interface stats twice to calculate throughput
	stats1, err := readNetDevStats()
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	stats2, err := readNetDevStats()
	if err != nil {
		return nil, err
	}

	for name, s1 := range stats1 {
		s2, ok := stats2[name]
		if !ok {
			continue
		}

		// Skip loopback
		if name == "lo" {
			continue
		}

		// Utilization (bytes/sec - we show rate, can't determine % without knowing max)
		rxRate := float64(s2.RxBytes-s1.RxBytes) * 10 // Scale to per-second
		txRate := float64(s2.TxBytes-s1.TxBytes) * 10
		totalRate := rxRate + txRate

		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Network (%s)", name),
			Type:        use.Utilization,
			Value:       formatBytes(totalRate) + "/s",
			RawValue:    totalRate,
			Status:      use.StatusOK, // Can't determine % without max bandwidth
			Description: "Network throughput",
			Command:     "/proc/net/dev",
		})

		// Saturation (dropped packets)
		drops := s2.RxDropped + s2.TxDropped
		dropStatus := use.StatusOK
		if drops > 0 {
			dropStatus = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Network (%s)", name),
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%d drops", drops),
			RawValue:    float64(drops),
			Status:      dropStatus,
			Description: "Dropped packets indicate network saturation",
			Command:     "/proc/net/dev",
		})

		// Errors
		errs := s2.RxErrors + s2.TxErrors
		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Network (%s)", name),
			Type:        use.Errors,
			Value:       fmt.Sprintf("%d", errs),
			RawValue:    float64(errs),
			Status:      use.EvaluateErrors(int64(errs)),
			Description: "Network interface errors",
			Command:     "/proc/net/dev",
		})
	}

	return checks, nil
}

// readNetDevStats reads network interface statistics from /proc/net/dev.
func readNetDevStats() (map[string]InterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]InterfaceStats)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		// Skip header lines
		if lineNum <= 2 {
			continue
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		s := InterfaceStats{Name: name}
		s.RxBytes, _ = strconv.ParseUint(fields[0], 10, 64)
		s.RxPackets, _ = strconv.ParseUint(fields[1], 10, 64)
		s.RxErrors, _ = strconv.ParseUint(fields[2], 10, 64)
		s.RxDropped, _ = strconv.ParseUint(fields[3], 10, 64)
		s.TxBytes, _ = strconv.ParseUint(fields[8], 10, 64)
		s.TxPackets, _ = strconv.ParseUint(fields[9], 10, 64)
		s.TxErrors, _ = strconv.ParseUint(fields[10], 10, 64)
		s.TxDropped, _ = strconv.ParseUint(fields[11], 10, 64)

		stats[name] = s
	}

	return stats, scanner.Err()
}

// formatBytes formats bytes into human-readable format.
func formatBytes(b float64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	div, exp := float64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", b/div, "KMGTPE"[exp])
}
