// Package workload provides workload characterization - answers "what is the system doing?"
package workload

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ProcessInfo holds information about a single process.
type ProcessInfo struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPUPct  float64 `json:"cpu_pct"`
	MemPct  float64 `json:"mem_pct"`
	Command string  `json:"command"`
	State   string  `json:"state"`
}

// Report holds the complete workload characterization.
type Report struct {
	TopCPUProcesses    []ProcessInfo  `json:"top_cpu_processes"`
	TopMemProcesses    []ProcessInfo  `json:"top_mem_processes"`
	TopIOProcesses     []ProcessInfo  `json:"top_io_processes,omitempty"`
	ProcessStateCounts map[string]int `json:"process_state_counts"`
	LoadAverages       [3]float64     `json:"load_averages"`
	LoadTrend          string         `json:"load_trend"`
	Summary            string         `json:"summary"`
}

var (
	wlTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	wlHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	wlDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	wlWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	wlOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
)

// Render outputs the workload report with lipgloss styling.
func (r *Report) Render(w io.Writer, topN int) {
	fmt.Fprintln(w, wlTitle.Render("Workload Characterization Report"))
	fmt.Fprintln(w, wlDim.Render(strings.Repeat("═", 60)))
	fmt.Fprintln(w)

	// Load averages
	trendStyle := wlOK
	if r.LoadTrend == "increasing" {
		trendStyle = wlWarn
	}
	fmt.Fprintf(w, "%s  %.2f, %.2f, %.2f  %s\n",
		wlTitle.Render("Load Averages:"),
		r.LoadAverages[0], r.LoadAverages[1], r.LoadAverages[2],
		trendStyle.Render("("+r.LoadTrend+")"))
	fmt.Fprintln(w)

	// Process states
	if len(r.ProcessStateCounts) > 0 {
		fmt.Fprintf(w, "%s  ", wlTitle.Render("Process States:"))
		parts := make([]string, 0)
		for state, count := range r.ProcessStateCounts {
			parts = append(parts, fmt.Sprintf("%s=%d", state, count))
		}
		fmt.Fprintln(w, strings.Join(parts, ", "))
		fmt.Fprintln(w)
	}

	// Top CPU
	if len(r.TopCPUProcesses) > 0 {
		fmt.Fprintln(w, wlTitle.Render("Top CPU Consumers"))
		fmt.Fprintf(w, "  %s %s %s %s\n",
			wlHeader.Render("PID     "),
			wlHeader.Render("USER       "),
			wlHeader.Render("CPU%   "),
			wlHeader.Render("COMMAND"))
		fmt.Fprintln(w, "  "+wlDim.Render(strings.Repeat("─", 60)))
		limit := topN
		if limit > len(r.TopCPUProcesses) {
			limit = len(r.TopCPUProcesses)
		}
		for _, p := range r.TopCPUProcesses[:limit] {
			fmt.Fprintf(w, "  %-8d %-12s %-8.1f %s\n", p.PID, p.User, p.CPUPct, p.Command)
		}
		fmt.Fprintln(w)
	}

	// Top Memory
	if len(r.TopMemProcesses) > 0 {
		fmt.Fprintln(w, wlTitle.Render("Top Memory Consumers"))
		fmt.Fprintf(w, "  %s %s %s %s\n",
			wlHeader.Render("PID     "),
			wlHeader.Render("USER       "),
			wlHeader.Render("MEM%   "),
			wlHeader.Render("COMMAND"))
		fmt.Fprintln(w, "  "+wlDim.Render(strings.Repeat("─", 60)))
		limit := topN
		if limit > len(r.TopMemProcesses) {
			limit = len(r.TopMemProcesses)
		}
		for _, p := range r.TopMemProcesses[:limit] {
			fmt.Fprintf(w, "  %-8d %-12s %-8.1f %s\n", p.PID, p.User, p.MemPct, p.Command)
		}
		fmt.Fprintln(w)
	}

	// Summary
	if r.Summary != "" {
		fmt.Fprintf(w, "%s %s\n", wlTitle.Render("Summary:"), r.Summary)
	}
}

// characterizeLoadTrend determines if load is increasing, decreasing, or stable.
func characterizeLoadTrend(load1, load5, load15 float64) string {
	if load1 > load5*1.2 && load5 > load15*1.2 {
		return "increasing"
	}
	if load1 < load5*0.8 && load5 < load15*0.8 {
		return "decreasing"
	}
	return "stable"
}
