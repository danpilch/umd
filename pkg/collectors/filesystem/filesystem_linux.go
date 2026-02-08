//go:build linux

package filesystem

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers filesystem USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0)

	// Utilization: inode usage per mount point
	mountPoints := getFilesystemMountPoints()
	for _, mp := range mountPoints {
		var stat unix.Statfs_t
		if err := unix.Statfs(mp, &stat); err != nil {
			continue
		}
		if stat.Files == 0 {
			continue
		}

		usedInodes := stat.Files - stat.Ffree
		inodePercent := (float64(usedInodes) / float64(stat.Files)) * 100

		status := thresholds.EvaluateUtilization(inodePercent)
		checks = append(checks, use.Check{
			Resource:    fmt.Sprintf("Filesystem (%s)", mp),
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.1f%% inodes", inodePercent),
			RawValue:    inodePercent,
			Status:      status,
			Description: fmt.Sprintf("Inode usage: %d/%d", usedInodes, stat.Files),
			Command:     "statfs",
		})

		// Errors: zero free inodes
		if stat.Ffree == 0 {
			checks = append(checks, use.Check{
				Resource:    fmt.Sprintf("Filesystem (%s)", mp),
				Type:        use.Errors,
				Value:       "0 free inodes",
				RawValue:    1,
				Status:      use.StatusError,
				Description: "No free inodes available",
				Command:     "statfs",
			})
		}
	}

	// Saturation: FD utilization from /proc/sys/fs/file-nr
	fdUtil, err := getFDUtilization()
	if err == nil {
		status := use.StatusOK
		if fdUtil > 70 {
			status = use.StatusWarning
		}
		if fdUtil > 90 {
			status = use.StatusError
		}
		checks = append(checks, use.Check{
			Resource:    "Filesystem (FDs)",
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%.1f%% FDs used", fdUtil),
			RawValue:    fdUtil,
			Status:      status,
			Description: "File descriptor utilization (allocated/max)",
			Command:     "/proc/sys/fs/file-nr",
		})
	}

	return checks, nil
}

func getFilesystemMountPoints() []string {
	points := []string{"/"}

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return points
	}
	defer file.Close()

	seen := map[string]bool{"/": true}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
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

		if !seen[mp] {
			seen[mp] = true
			points = append(points, mp)
		}
	}
	return points
}

func getFDUtilization() (float64, error) {
	data, err := os.ReadFile("/proc/sys/fs/file-nr")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, fmt.Errorf("unexpected file-nr format")
	}
	allocated, _ := strconv.ParseFloat(fields[0], 64)
	max, _ := strconv.ParseFloat(fields[2], 64)
	if max == 0 {
		return 0, fmt.Errorf("max FDs is 0")
	}
	return (allocated / max) * 100, nil
}
