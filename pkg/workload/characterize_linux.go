//go:build linux

package workload

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Characterize gathers workload information on Linux.
func Characterize() (*Report, error) {
	report := &Report{
		ProcessStateCounts: make(map[string]int),
	}

	// Load averages
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			report.LoadAverages[0], _ = strconv.ParseFloat(fields[0], 64)
			report.LoadAverages[1], _ = strconv.ParseFloat(fields[1], 64)
			report.LoadAverages[2], _ = strconv.ParseFloat(fields[2], 64)
		}
	}
	report.LoadTrend = characterizeLoadTrend(
		report.LoadAverages[0], report.LoadAverages[1], report.LoadAverages[2])

	// Read all processes from /proc/[pid]/stat
	procs, err := readAllProcesses()
	if err == nil {
		// Count process states
		for _, p := range procs {
			report.ProcessStateCounts[p.State]++
		}

		// Sort by CPU
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].CPUPct > procs[j].CPUPct
		})
		report.TopCPUProcesses = procs

		// Sort by memory
		memProcs := make([]ProcessInfo, len(procs))
		copy(memProcs, procs)
		sort.Slice(memProcs, func(i, j int) bool {
			return memProcs[i].MemPct > memProcs[j].MemPct
		})
		report.TopMemProcesses = memProcs
	}

	// Summary
	report.Summary = fmt.Sprintf("Load trend: %s. %d total processes.",
		report.LoadTrend, len(procs))

	return report, nil
}

func readAllProcesses() ([]ProcessInfo, error) {
	dirs, err := filepath.Glob("/proc/[0-9]*/stat")
	if err != nil {
		return nil, err
	}

	// Get total memory for percentage calculation
	totalMem := getTotalMemory()

	var procs []ProcessInfo
	for _, statPath := range dirs {
		p, err := readProcessStat(statPath, totalMem)
		if err != nil {
			continue
		}
		procs = append(procs, p)
	}
	return procs, nil
}

func readProcessStat(path string, totalMem uint64) (ProcessInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProcessInfo{}, err
	}

	content := string(data)
	// Find the command name (enclosed in parentheses)
	start := strings.Index(content, "(")
	end := strings.LastIndex(content, ")")
	if start < 0 || end < 0 {
		return ProcessInfo{}, fmt.Errorf("invalid stat format")
	}

	comm := content[start+1 : end]
	rest := strings.Fields(content[end+2:])
	pidStr := strings.Fields(content[:start])[0]
	pid, _ := strconv.Atoi(pidStr)

	if len(rest) < 22 {
		return ProcessInfo{}, fmt.Errorf("insufficient fields")
	}

	state := rest[0]
	// utime and stime are fields 13 and 14 (0-indexed from after comm)
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	// vsize is field 22, rss is field 23
	rss, _ := strconv.ParseUint(rest[21], 10, 64)
	rssBytes := rss * 4096 // pages to bytes

	// CPU% is approximate - based on total time
	cpuTicks := float64(utime + stime)
	// Normalize to approximate percentage (rough)
	cpuPct := cpuTicks / 100.0
	if cpuPct > 100 {
		cpuPct = 100
	}

	var memPct float64
	if totalMem > 0 {
		memPct = (float64(rssBytes) / float64(totalMem)) * 100
	}

	// Get user from /proc/[pid]/status
	user := getProcessUser(pid)

	return ProcessInfo{
		PID:     pid,
		User:    user,
		CPUPct:  cpuPct,
		MemPct:  memPct,
		Command: comm,
		State:   state,
	}, nil
}

func getProcessUser(pid int) string {
	path := fmt.Sprintf("/proc/%d/status", pid)
	file, err := os.Open(path)
	if err != nil {
		return "?"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return "?"
}

func getTotalMemory() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.TrimSuffix(fields[0], ":") == "MemTotal" {
			val, _ := strconv.ParseUint(fields[1], 10, 64)
			return val * 1024 // kB to bytes
		}
	}
	return 0
}
