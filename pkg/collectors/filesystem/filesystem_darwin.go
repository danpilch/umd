//go:build darwin

package filesystem

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers filesystem USE metrics on macOS.
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

		usedInodes := stat.Files - uint64(stat.Ffree)
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

	// Saturation: FD utilization from sysctl
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
			Description: "File descriptor utilization",
			Command:     "sysctl kern.maxfiles",
		})
	}

	return checks, nil
}

func getFilesystemMountPoints() []string {
	points := []string{"/"}

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
		if lineNum == 1 {
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}
		device := fields[0]
		mp := fields[5]

		if strings.HasPrefix(device, "devfs") ||
			strings.HasPrefix(device, "map ") ||
			device == "none" ||
			strings.HasPrefix(mp, "/System/Volumes/") {
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
	// Get current number of open files
	cmd := exec.Command("sysctl", "-n", "kern.num_files")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	numFiles, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}

	// Get max files
	cmd = exec.Command("sysctl", "-n", "kern.maxfiles")
	out, err = cmd.Output()
	if err != nil {
		return 0, err
	}
	maxFiles, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}

	if maxFiles == 0 {
		return 0, fmt.Errorf("maxfiles is 0")
	}
	return (numFiles / maxFiles) * 100, nil
}
