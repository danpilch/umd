package baseline

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/danpilch/umd/pkg/use"
)

// Severity indicates the magnitude of a metric drift.
type Severity string

const (
	SeverityNone     Severity = "none"
	SeverityMinor    Severity = "minor"
	SeverityModerate Severity = "moderate"
	SeverityMajor    Severity = "major"
	SeverityRegress  Severity = "regression"
)

// Comparison holds the drift analysis for a single metric.
type Comparison struct {
	Resource    string
	Type        use.MetricType
	BaselineVal float64
	CurrentVal  float64
	DeltaPct    float64
	Severity    Severity
}

var (
	blTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	blHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	blDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	blOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	blWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	blErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	blMinor   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

// Compare matches checks by Resource+Type and calculates drift.
func Compare(baseline *Baseline, current []use.Check) []Comparison {
	// Index baseline by resource+type
	baselineMap := make(map[string]use.Check)
	for _, c := range baseline.Checks {
		key := c.Resource + "|" + string(c.Type)
		baselineMap[key] = c
	}

	var comparisons []Comparison
	for _, cur := range current {
		key := cur.Resource + "|" + string(cur.Type)
		base, ok := baselineMap[key]
		if !ok {
			continue
		}

		var deltaPct float64
		if base.RawValue != 0 {
			deltaPct = ((cur.RawValue - base.RawValue) / math.Abs(base.RawValue)) * 100
		} else if cur.RawValue != 0 {
			deltaPct = 100
		}

		sev := classifySeverity(deltaPct)

		comparisons = append(comparisons, Comparison{
			Resource:    cur.Resource,
			Type:        cur.Type,
			BaselineVal: base.RawValue,
			CurrentVal:  cur.RawValue,
			DeltaPct:    deltaPct,
			Severity:    sev,
		})
	}

	return comparisons
}

func classifySeverity(deltaPct float64) Severity {
	absDelta := math.Abs(deltaPct)
	if absDelta < 5 {
		return SeverityNone
	}
	if absDelta < 15 {
		return SeverityMinor
	}
	if absDelta < 30 {
		return SeverityModerate
	}
	if deltaPct > 0 {
		return SeverityRegress
	}
	return SeverityMajor
}

// RenderComparison outputs a styled comparison table.
func RenderComparison(w io.Writer, baseline *Baseline, comparisons []Comparison) {
	fmt.Fprintln(w, blTitle.Render("Baseline Comparison"))
	fmt.Fprintln(w, blDim.Render(strings.Repeat("═", 90)))
	fmt.Fprintf(w, "Comparing against %s (from %s)\n\n",
		lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%q", baseline.Name)),
		blDim.Render(baseline.Timestamp.Format("2006-01-02 15:04:05")))

	fmt.Fprintf(w, "  %s %s %s %s %s %s\n",
		blHeader.Render("RESOURCE                "),
		blHeader.Render("TYPE          "),
		blHeader.Render("BASELINE  "),
		blHeader.Render("CURRENT   "),
		blHeader.Render("DELTA    "),
		blHeader.Render("SEVERITY  "))
	fmt.Fprintln(w, "  "+blDim.Render(strings.Repeat("─", 90)))

	regressions := 0
	for _, c := range comparisons {
		deltaStr := fmt.Sprintf("%+.1f%%", c.DeltaPct)
		var sevStr string
		switch c.Severity {
		case SeverityRegress:
			sevStr = blErr.Render("REGRESSION")
			regressions++
		case SeverityMajor:
			sevStr = blErr.Render("MAJOR")
			regressions++
		case SeverityModerate:
			sevStr = blWarn.Render("moderate")
		case SeverityMinor:
			sevStr = blMinor.Render("minor")
		default:
			sevStr = blOK.Render("none")
		}

		fmt.Fprintf(w, "  %-25s %-15s %-12.2f %-12.2f %-10s %s\n",
			c.Resource, c.Type, c.BaselineVal, c.CurrentVal, deltaStr, sevStr)
	}

	fmt.Fprintln(w)
	if regressions > 0 {
		fmt.Fprintf(w, "  %s\n", blErr.Render(fmt.Sprintf("%d potential regressions detected.", regressions)))
	} else {
		fmt.Fprintf(w, "  %s\n", blOK.Render("No significant regressions detected."))
	}
}
