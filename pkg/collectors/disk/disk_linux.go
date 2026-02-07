//go:build linux

package disk

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danpilch/umd/pkg/use"
)

// DiskStats holds disk I/O statistics from /proc/diskstats.
type DiskStats struct {
	Name            string
	ReadsCompleted  uint64
	ReadsMerged     uint64
	SectorsRead     uint64
	TimeReading     uint64
	WritesCompleted uint64
	WritesMerged    uint64
	SectorsWritten  uint64
	TimeWriting     uint64
	IOsInProgress   uint64
	TimeDoingIO     uint64
	WeightedTime    uint64
}

// Collect gathers disk USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0)

	// Get disk I/O stats
	stats1, err := readDiskStats()
	if err != nil {
		return nil, err
	}

	time.Sleep(100 * time.Millisecond)

	stats2, err := readDiskStats()
	if err != nil {
		return nil, err
	}

	for name, s1 := range stats1 {
		s2, ok := stats2[name]
		if !ok {
			continue
		}

		// Skip partitions (only show whole disks like sda, nvme0n1)
		if isPartition(name) {
			continue
		}

		// Utilization (% time doing I/O)
		timeDelta := float64(s2.TimeDoingIO - s1.TimeDoingIO)
		// 100ms = 100000 microseconds, TimeDoingIO is in milliseconds
		utilPercent := timeDelta / 100.0 // Convert to percentage

		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Disk (%s)", name),
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.1f%%", utilPercent),
			RawValue:    utilPercent,
			Status:      thresholds.EvaluateUtilization(utilPercent),
			Description: "I/O busy percentage",
			Command:     "/proc/diskstats",
		})

		// Saturation (average queue size)
		weightedDelta := float64(s2.WeightedTime - s1.WeightedTime)
		avgQueue := weightedDelta / 100.0 // Normalize to seconds

		satStatus := use.StatusOK
		if avgQueue > 1.0 {
			satStatus = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Disk (%s)", name),
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%.2f avgqu", avgQueue),
			RawValue:    avgQueue,
			Status:      satStatus,
			Description: "Average queue size",
			Command:     "/proc/diskstats",
		})

		// Errors (from /sys)
		errCount := getIOErrors(name)
		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Disk (%s)", name),
			Type:        use.Errors,
			Value:       fmt.Sprintf("%d", errCount),
			RawValue:    float64(errCount),
			Status:      use.EvaluateErrors(errCount),
			Description: "I/O errors",
			Command:     "/sys/block/*/device/ioerr_cnt",
		})
	}

	// Add filesystem capacity checks
	mountPoints := getMainMountPoints()
	checks = append(checks, GetFilesystemChecks(thresholds, mountPoints)...)

	return checks, nil
}

// readDiskStats reads disk statistics from /proc/diskstats.
func readDiskStats() (map[string]DiskStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]DiskStats)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		name := fields[2]
		s := DiskStats{Name: name}
		s.ReadsCompleted, _ = strconv.ParseUint(fields[3], 10, 64)
		s.ReadsMerged, _ = strconv.ParseUint(fields[4], 10, 64)
		s.SectorsRead, _ = strconv.ParseUint(fields[5], 10, 64)
		s.TimeReading, _ = strconv.ParseUint(fields[6], 10, 64)
		s.WritesCompleted, _ = strconv.ParseUint(fields[7], 10, 64)
		s.WritesMerged, _ = strconv.ParseUint(fields[8], 10, 64)
		s.SectorsWritten, _ = strconv.ParseUint(fields[9], 10, 64)
		s.TimeWriting, _ = strconv.ParseUint(fields[10], 10, 64)
		s.IOsInProgress, _ = strconv.ParseUint(fields[11], 10, 64)
		s.TimeDoingIO, _ = strconv.ParseUint(fields[12], 10, 64)
		s.WeightedTime, _ = strconv.ParseUint(fields[13], 10, 64)

		stats[name] = s
	}

	return stats, scanner.Err()
}

// isPartition returns true if the disk name appears to be a partition.
func isPartition(name string) bool {
	// Check for partition number suffix
	if len(name) == 0 {
		return false
	}

	// Handle sdXN (e.g., sda1), hdXN, vdXN
	if (strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "hd") || strings.HasPrefix(name, "vd")) && len(name) > 3 {
		lastChar := name[len(name)-1]
		return lastChar >= '0' && lastChar <= '9'
	}

	// Handle nvmeXnYpZ (e.g., nvme0n1p1)
	if strings.HasPrefix(name, "nvme") && strings.Contains(name, "p") {
		return true
	}

	// Handle mmcblkXpY
	if strings.HasPrefix(name, "mmcblk") && strings.Contains(name, "p") {
		return true
	}

	// Handle loop devices
	if strings.HasPrefix(name, "loop") {
		return true
	}

	// Handle ram devices
	if strings.HasPrefix(name, "ram") {
		return true
	}

	return false
}

// getIOErrors reads I/O error count from /sys.
func getIOErrors(diskName string) int64 {
	path := fmt.Sprintf("/sys/block/%s/device/ioerr_cnt", diskName)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	count, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return count
}

// getMainMountPoints returns the main mount points to check.
func getMainMountPoints() []string {
	// Always check root
	points := []string{"/"}

	// Read /proc/mounts and add common important mount points
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return points
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	seen := map[string]bool{"/": true}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		mp := fields[1]
		fstype := fields[2]

		// Skip virtual filesystems
		if fstype == "tmpfs" || fstype == "devtmpfs" || fstype == "sysfs" ||
			fstype == "proc" || fstype == "devpts" || fstype == "cgroup" ||
			fstype == "cgroup2" || fstype == "securityfs" || fstype == "debugfs" ||
			fstype == "tracefs" || fstype == "configfs" || fstype == "fusectl" ||
			fstype == "hugetlbfs" || fstype == "mqueue" || fstype == "pstore" {
			continue
		}

		// Add if not already seen
		if !seen[mp] {
			seen[mp] = true
			points = append(points, mp)
		}
	}

	return points
}
