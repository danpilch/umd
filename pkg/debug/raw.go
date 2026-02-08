package debug

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/danpilch/umd/pkg/use"
)

// DumpRawMetrics outputs all collected checks with raw values before threshold evaluation.
func DumpRawMetrics(w io.Writer, checks []use.Check) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	fmt.Fprintln(w)
	fmt.Fprintln(w, title.Render("Raw Metrics Dump"))
	fmt.Fprintln(w, dim.Render(strings.Repeat("═", 85)))
	fmt.Fprintf(w, "  %s %s %s %s %s\n",
		header.Render("RESOURCE                "),
		header.Render("TYPE          "),
		header.Render("RAW VALUE     "),
		header.Render("VALUE      "),
		header.Render("COMMAND   "))
	fmt.Fprintln(w, "  "+dim.Render(strings.Repeat("─", 85)))

	for _, c := range checks {
		fmt.Fprintf(w, "  %-25s %-15s %-15.4f %-12s %s\n",
			c.Resource, c.Type, c.RawValue, c.Value, dim.Render(c.Command))
	}
}
