//go:build darwin

package network

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
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

// Collect gathers network USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0)

	// Get interface stats twice to calculate throughput
	stats1, err := readNetstatStats()
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	stats2, err := readNetstatStats()
	if err != nil {
		return nil, err
	}

	for name, s1 := range stats1 {
		s2, ok := stats2[name]
		if !ok {
			continue
		}

		// Skip loopback and virtual interfaces
		if name == "lo0" || !isPhysicalInterface(name) {
			continue
		}

		// Skip interfaces with no traffic
		if s2.RxBytes == 0 && s2.TxBytes == 0 {
			continue
		}

		// Utilization (bytes/sec)
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
			Command:     "netstat -ib",
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
			Command:     "netstat -ib",
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
			Command:     "netstat -ib",
		})
	}

	return checks, nil
}

// readNetstatStats reads network interface statistics from netstat -ib.
func readNetstatStats() (map[string]InterfaceStats, error) {
	cmd := exec.Command("netstat", "-ib")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]InterfaceStats)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		// Skip header line
		if lineNum == 1 {
			continue
		}

		fields := strings.Fields(scanner.Text())
		// Name Mtu Network Address Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll Drop
		if len(fields) < 11 {
			continue
		}

		name := fields[0]

		// Skip duplicate entries for the same interface (IPv4/IPv6)
		if _, exists := stats[name]; exists {
			continue
		}

		s := InterfaceStats{Name: name}
		s.RxPackets, _ = strconv.ParseUint(fields[4], 10, 64)
		s.RxErrors, _ = strconv.ParseUint(fields[5], 10, 64)
		s.RxBytes, _ = strconv.ParseUint(fields[6], 10, 64)
		s.TxPackets, _ = strconv.ParseUint(fields[7], 10, 64)
		s.TxErrors, _ = strconv.ParseUint(fields[8], 10, 64)
		s.TxBytes, _ = strconv.ParseUint(fields[9], 10, 64)

		// Drop is the last column
		if len(fields) >= 12 {
			drops, _ := strconv.ParseUint(fields[11], 10, 64)
			s.RxDropped = drops / 2
			s.TxDropped = drops / 2
		}

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

// isPhysicalInterface returns true if the interface appears to be a physical network interface.
func isPhysicalInterface(name string) bool {
	// Skip known virtual/internal interfaces on macOS
	skipPrefixes := []string{
		"utun",   // VPN tunnels
		"anpi",   // Apple Network Protocol Interface
		"awdl",   // Apple Wireless Direct Link
		"llw",    // Low Latency WLAN
		"ap",     // Access Point
		"bridge", // Bridge interfaces
		"gif",    // Generic tunnel
		"stf",    // 6to4 tunnel
		"p2p",    // Point to point
	}

	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}

	return true
}
