//go:build darwin

package workload

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Characterize gathers workload information on macOS.
func Characterize() (*Report, error) {
	report := &Report{
		ProcessStateCounts: make(map[string]int),
	}

	// Load averages from sysctl
	if out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output(); err == nil {
		str := strings.Trim(string(out), "{ }\n")
		fields := strings.Fields(str)
		if len(fields) >= 3 {
			report.LoadAverages[0], _ = strconv.ParseFloat(fields[0], 64)
			report.LoadAverages[1], _ = strconv.ParseFloat(fields[1], 64)
			report.LoadAverages[2], _ = strconv.ParseFloat(fields[2], 64)
		}
	}
	report.LoadTrend = characterizeLoadTrend(
		report.LoadAverages[0], report.LoadAverages[1], report.LoadAverages[2])

	// Process listing from ps
	cpuProcs, err := getProcessesSortedBy("cpu")
	if err == nil {
		report.TopCPUProcesses = cpuProcs
	}

	memProcs, err := getProcessesSortedBy("mem")
	if err == nil {
		report.TopMemProcesses = memProcs
	}

	// Process states from ps ax
	states, err := getProcessStates()
	if err == nil {
		report.ProcessStateCounts = states
	}

	total := 0
	for _, c := range report.ProcessStateCounts {
		total += c
	}
	report.Summary = fmt.Sprintf("Load trend: %s. %d total processes.", report.LoadTrend, total)

	return report, nil
}

func getProcessesSortedBy(sortKey string) ([]ProcessInfo, error) {
	// ps aux sorted by cpu or mem
	cmd := exec.Command("ps", "aux")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var procs []ProcessInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // skip header
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 11 {
			continue
		}

		pid, _ := strconv.Atoi(fields[1])
		cpuPct, _ := strconv.ParseFloat(fields[2], 64)
		memPct, _ := strconv.ParseFloat(fields[3], 64)
		command := strings.Join(fields[10:], " ")

		procs = append(procs, ProcessInfo{
			PID:     pid,
			User:    fields[0],
			CPUPct:  cpuPct,
			MemPct:  memPct,
			Command: command,
			State:   fields[7],
		})
	}

	// Sort by the requested key
	if sortKey == "cpu" {
		sortByCPU(procs)
	} else {
		sortByMem(procs)
	}

	return procs, nil
}

func sortByCPU(procs []ProcessInfo) {
	for i := 1; i < len(procs); i++ {
		for j := i; j > 0 && procs[j].CPUPct > procs[j-1].CPUPct; j-- {
			procs[j], procs[j-1] = procs[j-1], procs[j]
		}
	}
}

func sortByMem(procs []ProcessInfo) {
	for i := 1; i < len(procs); i++ {
		for j := i; j > 0 && procs[j].MemPct > procs[j-1].MemPct; j-- {
			procs[j], procs[j-1] = procs[j-1], procs[j]
		}
	}
}

func getProcessStates() (map[string]int, error) {
	cmd := exec.Command("ps", "ax", "-o", "state")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	states := make(map[string]int)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue
		}
		state := strings.TrimSpace(scanner.Text())
		if len(state) > 0 {
			// Use just the first character for state classification
			states[string(state[0])]++
		}
	}
	return states, nil
}
