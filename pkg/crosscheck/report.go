package crosscheck

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/danpilch/umd/pkg/use"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	validStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	suspectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	conflictStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	passStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	failStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// Report outputs cross-check validation results and sanity checks as a styled table.
func Report(w io.Writer, validations []ValidationResult, sanity []SanityResult) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, titleStyle.Render("Cross-Check Validation Report"))
	fmt.Fprintln(w, dimStyle.Render(strings.Repeat("═", 60)))

	if len(validations) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, titleStyle.Render("Metric Cross-Checks"))
		fmt.Fprintf(w, "  %-25s %-12s %-12s %-10s %s\n",
			headerStyle.Render("METRIC"), headerStyle.Render("CONSENSUS"),
			headerStyle.Render("MAX DEV"), headerStyle.Render("STATUS"),
			headerStyle.Render("SOURCES"))
		fmt.Fprintln(w, "  "+dimStyle.Render(strings.Repeat("─", 80)))

		for _, v := range validations {
			sourceNames := make([]string, len(v.Sources))
			for i, s := range v.Sources {
				sourceNames[i] = fmt.Sprintf("%s=%.1f", s.Name, s.Value)
			}
			var statusStr string
			switch v.Status {
			case StatusConflict:
				statusStr = conflictStyle.Render("CONFLICT")
			case StatusSuspect:
				statusStr = suspectStyle.Render("SUSPECT")
			default:
				statusStr = validStyle.Render("VALID")
			}
			fmt.Fprintf(w, "  %-25s %-12.1f %-12.1f%% %-10s %s\n",
				v.Metric, v.Consensus, v.MaxDeviation, statusStr,
				dimStyle.Render(strings.Join(sourceNames, ", ")))
		}
	}

	if len(sanity) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, titleStyle.Render("Sanity Checks"))
		failed := 0
		for _, s := range sanity {
			var icon string
			if s.Passed {
				icon = passStyle.Render("PASS")
			} else {
				icon = failStyle.Render("FAIL")
				failed++
			}
			fmt.Fprintf(w, "  [%s] %-40s %s\n", icon, s.Check, dimStyle.Render(s.Details))
		}
		fmt.Fprintln(w)
		if failed == 0 {
			fmt.Fprintf(w, "  %s\n", passStyle.Render(fmt.Sprintf("All %d sanity checks passed.", len(sanity))))
		} else {
			fmt.Fprintf(w, "  %s\n", failStyle.Render(fmt.Sprintf("%d of %d sanity checks failed.", failed, len(sanity))))
		}
	}
}

// ReportJSON outputs cross-check results as JSON.
func ReportJSON(w io.Writer, validations []ValidationResult, sanity []SanityResult) error {
	output := struct {
		Validations []ValidationResult `json:"validations"`
		Sanity      []SanityResult     `json:"sanity"`
	}{
		Validations: validations,
		Sanity:      sanity,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// RunCrossChecks performs full cross-validation on collected checks.
func RunCrossChecks(checks []use.Check) ([]ValidationResult, []SanityResult) {
	validator := NewValidator()

	// Get alternative sources for cross-checking
	cpuSources := GetCPUSources()
	memSources := GetMemorySources()

	var validations []ValidationResult

	if len(cpuSources) > 0 {
		validations = append(validations, validator.CrossCheck("CPU Utilization", cpuSources))
	}
	if len(memSources) > 0 {
		validations = append(validations, validator.CrossCheck("Memory Utilization", memSources))
	}

	// Run sanity checks on all collected metrics
	sanity := RunSanityChecks(checks)

	return validations, sanity
}
