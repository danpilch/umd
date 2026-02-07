// Package disk provides disk I/O and capacity metrics collection for the USE method.
package disk

import (
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/danpilch/umd/pkg/use"
)

// Collector gathers disk-related USE metrics.
type Collector struct{}

// New creates a new disk collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "Disk"
}

// Filesystem represents a mounted filesystem.
type Filesystem struct {
	Device     string
	MountPoint string
	Total      uint64
	Used       uint64
	Available  uint64
}

// GetFilesystemUsage returns filesystem capacity metrics using statfs.
// This is cross-platform (works on both Linux and macOS).
func GetFilesystemUsage(mountPoint string) (*Filesystem, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(mountPoint, &stat); err != nil {
		return nil, err
	}

	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	available := stat.Bavail * blockSize
	used := total - (stat.Bfree * blockSize)

	return &Filesystem{
		MountPoint: mountPoint,
		Total:      total,
		Used:       used,
		Available:  available,
	}, nil
}

// GetFilesystemChecks returns USE checks for filesystem capacity.
func GetFilesystemChecks(thresholds use.Thresholds, mountPoints []string) []use.Check {
	checks := make([]use.Check, 0)

	for _, mp := range mountPoints {
		fs, err := GetFilesystemUsage(mp)
		if err != nil {
			continue
		}

		if fs.Total == 0 {
			continue
		}

		utilPercent := (float64(fs.Used) / float64(fs.Total)) * 100
		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Filesystem (%s)", mp),
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.1f%%", utilPercent),
			RawValue:    utilPercent,
			Status:      thresholds.EvaluateUtilization(utilPercent),
			Description: fmt.Sprintf("Used: %s / Total: %s", formatBytes(fs.Used), formatBytes(fs.Total)),
			Command:     "statfs",
		})
	}

	return checks
}

// formatBytes formats bytes into human-readable format.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
