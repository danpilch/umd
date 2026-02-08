package debug

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danpilch/umd/pkg/use"
)

var (
	debugTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	debugHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	debugDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// CollectorTiming records the duration of a collector's Collect call.
type CollectorTiming struct {
	Name     string
	Duration time.Duration
}

// TimedCollector wraps a use.Collector to record collection duration.
type TimedCollector struct {
	inner  use.Collector
	Timing CollectorTiming
}

// NewTimedCollector wraps a collector with timing instrumentation.
func NewTimedCollector(c use.Collector) *TimedCollector {
	return &TimedCollector{
		inner: c,
	}
}

// Name returns the wrapped collector's name.
func (t *TimedCollector) Name() string {
	return t.inner.Name()
}

// Collect runs the wrapped collector and records duration.
func (t *TimedCollector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	start := time.Now()
	checks, err := t.inner.Collect(thresholds)
	t.Timing = CollectorTiming{
		Name:     t.inner.Name(),
		Duration: time.Since(start),
	}
	return checks, err
}

// TimingReport prints a styled timing summary for all timed collectors.
func TimingReport(w io.Writer, timings []CollectorTiming) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, debugTitle.Render("Collector Timing Report"))
	fmt.Fprintln(w, debugDim.Render(strings.Repeat("═", 40)))
	fmt.Fprintf(w, "  %s  %s\n",
		debugHeader.Render("COLLECTOR          "),
		debugHeader.Render("DURATION    "))
	fmt.Fprintln(w, "  "+debugDim.Render(strings.Repeat("─", 40)))

	var total time.Duration
	for _, t := range timings {
		fmt.Fprintf(w, "  %-20s %v\n", t.Name, t.Duration)
		total += t.Duration
	}
	fmt.Fprintln(w, "  "+debugDim.Render(strings.Repeat("─", 40)))
	fmt.Fprintf(w, "  %-20s %v\n",
		lipgloss.NewStyle().Bold(true).Render("TOTAL"), total)
}
