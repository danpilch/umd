// Package output provides formatters for displaying USE method results.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/danpilch/umd/pkg/use"
)

// Format represents the output format type.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatAI    Format = "ai"
	FormatTSV   Format = "tsv"
)

// Formatter handles output formatting.
type Formatter struct {
	format    Format
	writer    io.Writer
	sparkline *SparklineTracker
	showScore bool
}

// NewFormatter creates a new formatter.
func NewFormatter(format Format, writer io.Writer) *Formatter {
	return &Formatter{
		format: format,
		writer: writer,
	}
}

// SetSparklineTracker enables sparkline tracking for watch mode.
func (f *Formatter) SetSparklineTracker(s *SparklineTracker) {
	f.sparkline = s
}

// SetShowScore enables health score display.
func (f *Formatter) SetShowScore(show bool) {
	f.showScore = show
}

// Render outputs the checks in the configured format.
func (f *Formatter) Render(checks []use.Check) error {
	// Record sparkline data if tracker is set
	if f.sparkline != nil {
		for _, c := range checks {
			key := c.Resource + "|" + string(c.Type)
			f.sparkline.Record(key, c.RawValue)
		}
	}

	switch f.format {
	case FormatJSON:
		return f.renderJSON(checks)
	case FormatAI:
		return f.renderAI(checks)
	case FormatTSV:
		return f.renderTSV(checks)
	default:
		return f.renderTable(checks)
	}
}

// renderJSON outputs checks as JSON.
func (f *Formatter) renderJSON(checks []use.Check) error {
	output := struct {
		Checks  []use.Check `json:"checks"`
		Summary use.Summary `json:"summary"`
	}{
		Checks:  checks,
		Summary: use.Summarize(checks),
	}

	enc := json.NewEncoder(f.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// renderTable outputs checks as a styled table.
func (f *Formatter) renderTable(checks []use.Check) error {
	// Define styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 1)

	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	// Status colors
	statusStyles := map[use.Status]lipgloss.Style{
		use.StatusOK:      lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true), // Green
		use.StatusWarning: lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true), // Yellow
		use.StatusError:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),  // Red
		use.StatusUnknown: lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true),  // Gray
	}

	// Print header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	fmt.Fprintln(f.writer, titleStyle.Render("USE Method System Check"))
	fmt.Fprintln(f.writer, strings.Repeat("═", 60))
	fmt.Fprintln(f.writer)

	// Build table data - add sparkline column if tracker is set
	hasSparklines := f.sparkline != nil
	rows := make([][]string, len(checks))
	for i, check := range checks {
		statusStyle := statusStyles[check.Status]
		row := []string{
			check.Resource,
			string(check.Type),
			check.Value,
			statusStyle.Render(strings.ToUpper(string(check.Status))),
		}
		if hasSparklines {
			key := check.Resource + "|" + string(check.Type)
			row = append(row, f.sparkline.Sparkline(key))
		}
		rows[i] = row
	}

	// Create table
	headers := []string{"RESOURCE", "TYPE", "VALUE", "STATUS"}
	if hasSparklines {
		headers = append(headers, "TREND")
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(rows...)

	fmt.Fprintln(f.writer, t)

	// Print summary
	summary := use.Summarize(checks)
	fmt.Fprintln(f.writer)
	f.renderSummary(summary, statusStyles)

	// Show health score if enabled
	if f.showScore {
		score := HealthScore(checks)
		label := ScoreLabel(score)
		scoreStyle := statusStyles[use.StatusOK]
		if score < 80 {
			scoreStyle = statusStyles[use.StatusWarning]
		}
		if score < 50 {
			scoreStyle = statusStyles[use.StatusError]
		}
		fmt.Fprintf(f.writer, "Health Score: %s\n",
			scoreStyle.Render(fmt.Sprintf("%d/100 (%s)", score, label)))
	}

	return nil
}

// renderSummary outputs the summary line.
func (f *Formatter) renderSummary(summary use.Summary, styles map[use.Status]lipgloss.Style) {
	parts := []string{}

	if summary.Errors > 0 {
		parts = append(parts, styles[use.StatusError].Render(fmt.Sprintf("%d errors", summary.Errors)))
	}
	if summary.Warnings > 0 {
		parts = append(parts, styles[use.StatusWarning].Render(fmt.Sprintf("%d warnings", summary.Warnings)))
	}
	if summary.Unknown > 0 {
		parts = append(parts, styles[use.StatusUnknown].Render(fmt.Sprintf("%d unknown", summary.Unknown)))
	}

	if len(parts) == 0 {
		fmt.Fprintln(f.writer, styles[use.StatusOK].Render("All checks passed"))
	} else {
		fmt.Fprintf(f.writer, "Summary: %s\n", strings.Join(parts, ", "))
	}
}

// renderAI outputs checks in an LLM-friendly format.
func (f *Formatter) renderAI(checks []use.Check) error {
	summary := use.Summarize(checks)

	// Quick status line
	if summary.Errors == 0 && summary.Warnings == 0 {
		fmt.Fprintln(f.writer, "# System Health: OK")
		fmt.Fprintln(f.writer, "\nAll USE method checks passed. No issues detected.")
		fmt.Fprintln(f.writer)
	} else {
		fmt.Fprintln(f.writer, "# System Health: Issues Detected")
		fmt.Fprintf(f.writer, "\n**Status:** %d errors, %d warnings, %d ok\n\n",
			summary.Errors, summary.Warnings, summary.OK)
	}

	// Group checks by resource
	resourceChecks := make(map[string][]use.Check)
	var resourceOrder []string
	for _, check := range checks {
		if _, exists := resourceChecks[check.Resource]; !exists {
			resourceOrder = append(resourceOrder, check.Resource)
		}
		resourceChecks[check.Resource] = append(resourceChecks[check.Resource], check)
	}

	// Issues section - only if there are problems
	issues := filterByStatus(checks, use.StatusError, use.StatusWarning)
	if len(issues) > 0 {
		fmt.Fprintln(f.writer, "## Issues Requiring Attention")
		fmt.Fprintln(f.writer)
		for _, check := range issues {
			severity := "WARNING"
			if check.Status == use.StatusError {
				severity = "ERROR"
			}
			fmt.Fprintf(f.writer, "- **[%s] %s %s:** %s\n",
				severity, check.Resource, check.Type, check.Value)
			fmt.Fprintf(f.writer, "  - %s\n", getAIInterpretation(check))
		}
		fmt.Fprintln(f.writer)
	}

	// Metrics summary table
	fmt.Fprintln(f.writer, "## All Metrics")
	fmt.Fprintln(f.writer)
	fmt.Fprintln(f.writer, "| Resource | Utilization | Saturation | Errors |")
	fmt.Fprintln(f.writer, "|----------|-------------|------------|--------|")

	for _, resource := range resourceOrder {
		rChecks := resourceChecks[resource]
		util, sat, errs := "-", "-", "-"
		for _, c := range rChecks {
			val := c.Value
			if c.Status != use.StatusOK {
				val = fmt.Sprintf("**%s**", val)
			}
			switch c.Type {
			case use.Utilization:
				util = val
			case use.Saturation:
				sat = val
			case use.Errors:
				errs = val
			}
		}
		fmt.Fprintf(f.writer, "| %s | %s | %s | %s |\n", resource, util, sat, errs)
	}
	fmt.Fprintln(f.writer)

	// Context section
	fmt.Fprintln(f.writer, "## Interpretation Guide")
	fmt.Fprintln(f.writer)
	fmt.Fprintln(f.writer, "- **Utilization**: How busy the resource is (high = near capacity)")
	fmt.Fprintln(f.writer, "- **Saturation**: Work waiting/queued (non-zero = resource is bottleneck)")
	fmt.Fprintln(f.writer, "- **Errors**: Hardware/software errors (any > 0 needs investigation)")
	fmt.Fprintln(f.writer)
	fmt.Fprintln(f.writer, "Thresholds: Warning ≥70%, Critical ≥90% for utilization metrics.")

	// Drill-down suggestions for issues
	suggestions := GetDrillDownSuggestions(checks)
	if len(suggestions) > 0 {
		fmt.Fprintln(f.writer)
		fmt.Fprintln(f.writer, "## Suggested Next Steps")
		fmt.Fprintln(f.writer)
		for metric, suggs := range suggestions {
			fmt.Fprintf(f.writer, "**%s:**\n", metric)
			for _, s := range suggs {
				fmt.Fprintf(f.writer, "- `%s` - %s\n", s.Command, s.Reason)
			}
			fmt.Fprintln(f.writer)
		}
	}

	return nil
}

// renderTSV outputs checks as tab-separated values.
func (f *Formatter) renderTSV(checks []use.Check) error {
	// Header
	fmt.Fprintln(f.writer, "RESOURCE\tTYPE\tVALUE\tRAW_VALUE\tSTATUS\tDESCRIPTION\tCOMMAND")

	for _, c := range checks {
		fmt.Fprintf(f.writer, "%s\t%s\t%s\t%.4f\t%s\t%s\t%s\n",
			c.Resource, c.Type, c.Value, c.RawValue,
			c.Status, c.Description, c.Command)
	}

	return nil
}

// filterByStatus returns checks matching any of the given statuses.
func filterByStatus(checks []use.Check, statuses ...use.Status) []use.Check {
	var result []use.Check
	statusSet := make(map[use.Status]bool)
	for _, s := range statuses {
		statusSet[s] = true
	}
	for _, c := range checks {
		if statusSet[c.Status] {
			result = append(result, c)
		}
	}
	return result
}

// getAIInterpretation returns actionable context for a check.
func getAIInterpretation(check use.Check) string {
	resource := strings.ToLower(check.Resource)

	switch check.Type {
	case use.Utilization:
		if check.RawValue >= 90 {
			return "Critical: Resource near capacity. Immediate attention needed."
		}
		return "Elevated usage. Monitor for sustained high values."

	case use.Saturation:
		if strings.Contains(resource, "cpu") {
			return "Load exceeds CPU count. Processes waiting for CPU time."
		}
		if strings.Contains(resource, "memory") {
			return "System swapping/paging. Memory pressure detected."
		}
		if strings.Contains(resource, "disk") {
			return "I/O queue building up. Storage may be bottleneck."
		}
		if strings.Contains(resource, "network") {
			return "Packets being dropped. Network congestion or buffer issues."
		}
		return "Resource has queued work. Potential bottleneck."

	case use.Errors:
		if strings.Contains(resource, "cpu") {
			return "CPU errors in kernel log. Check for hardware issues or thermal throttling."
		}
		if strings.Contains(resource, "memory") {
			return "Memory pressure events (OOM/jetsam). Applications may have been killed."
		}
		if strings.Contains(resource, "disk") {
			return "Storage I/O errors. Check disk health with SMART data."
		}
		if strings.Contains(resource, "network") {
			return "Network interface errors. Check cables, drivers, or hardware."
		}
		return "Errors detected. Review system logs for details."
	}

	return check.Description
}
