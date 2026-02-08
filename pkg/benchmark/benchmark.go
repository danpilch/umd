// Package benchmark provides self-benchmarking to validate tool overhead.
package benchmark

import (
	"fmt"
	"io"
	"math"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danpilch/umd/pkg/use"
)

// Options configures a benchmark run.
type Options struct {
	Iterations int
	Warmup     int
}

// DefaultOptions returns sensible benchmark defaults.
func DefaultOptions() Options {
	return Options{
		Iterations: 20,
		Warmup:     3,
	}
}

// Result holds benchmark results for a single collector.
type Result struct {
	Collector   string
	Latencies   []time.Duration
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	ValueStdDev float64
}

// Overhead holds the tool's own resource usage.
type Overhead struct {
	AllocBytes uint64
	AllocCount uint64
	GCPauses   uint32
}

var (
	bmTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	bmHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	bmDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// Run benchmarks each collector with the given options.
func Run(collectors []use.Collector, thresholds use.Thresholds, opts Options) []Result {
	var results []Result

	for _, col := range collectors {
		// Warmup
		for i := 0; i < opts.Warmup; i++ {
			col.Collect(thresholds)
		}

		// Benchmark
		latencies := make([]time.Duration, opts.Iterations)
		var values []float64

		for i := 0; i < opts.Iterations; i++ {
			start := time.Now()
			checks, err := col.Collect(thresholds)
			latencies[i] = time.Since(start)

			if err == nil {
				for _, c := range checks {
					values = append(values, c.RawValue)
				}
			}
		}

		// Sort latencies for percentile calculation
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		result := Result{
			Collector:   col.Name(),
			Latencies:   latencies,
			P50:         percentile(latencies, 0.50),
			P95:         percentile(latencies, 0.95),
			P99:         percentile(latencies, 0.99),
			ValueStdDev: stddev(values),
		}
		results = append(results, result)
	}

	return results
}

// MeasureOverhead returns the tool's CPU and memory overhead.
func MeasureOverhead() Overhead {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return Overhead{
		AllocBytes: m.TotalAlloc,
		AllocCount: m.Mallocs,
		GCPauses:   m.NumGC,
	}
}

// RenderResults outputs styled benchmark results.
func RenderResults(w io.Writer, results []Result, overhead Overhead) {
	fmt.Fprintln(w, bmTitle.Render("Self-Benchmark Results"))
	fmt.Fprintln(w, bmDim.Render(strings.Repeat("═", 70)))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s %s %s %s %s\n",
		bmHeader.Render("COLLECTOR          "),
		bmHeader.Render("P50        "),
		bmHeader.Render("P95        "),
		bmHeader.Render("P99        "),
		bmHeader.Render("VALUE STDDEV"))
	fmt.Fprintln(w, "  "+bmDim.Render(strings.Repeat("─", 70)))

	for _, r := range results {
		fmt.Fprintf(w, "  %-20s %-12v %-12v %-12v %.4f\n",
			r.Collector, r.P50, r.P95, r.P99, r.ValueStdDev)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, bmTitle.Render("Tool Overhead"))
	fmt.Fprintln(w, bmDim.Render(strings.Repeat("─", 40)))
	fmt.Fprintf(w, "  Memory allocated: %s\n", lipgloss.NewStyle().Bold(true).Render(formatBytes(overhead.AllocBytes)))
	fmt.Fprintf(w, "  Allocations:      %s\n", lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", overhead.AllocCount)))
	fmt.Fprintf(w, "  GC pauses:        %s\n", lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", overhead.GCPauses)))
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sum, sumSq float64
	for _, v := range values {
		sum += v
		sumSq += v * v
	}
	n := float64(len(values))
	mean := sum / n
	variance := (sumSq / n) - (mean * mean)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
