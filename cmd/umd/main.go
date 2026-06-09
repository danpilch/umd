// Package main provides the CLI entry point for the USE Method Daemon (umd).
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/danpilch/umd/pkg/collectors/cpu"
	"github.com/danpilch/umd/pkg/collectors/disk"
	"github.com/danpilch/umd/pkg/collectors/filesystem"
	"github.com/danpilch/umd/pkg/collectors/memory"
	"github.com/danpilch/umd/pkg/collectors/network"
	"github.com/danpilch/umd/pkg/collectors/scheduler"
	"github.com/danpilch/umd/pkg/collectors/tcp"
	"github.com/danpilch/umd/pkg/collectors/vmem"
	"github.com/danpilch/umd/pkg/crosscheck"
	"github.com/danpilch/umd/pkg/debug"
	"github.com/danpilch/umd/pkg/output"
	"github.com/danpilch/umd/pkg/use"
)

var (
	// CLI flags
	formatFlag     string
	resourceFlag   string
	watchFlag      bool
	intervalFlag   int
	warnUtil       float64
	critUtil       float64
	verboseFlag    bool
	crosscheckFlag bool
	pprofFlag      bool
	traceFlag      bool
	rawFlag        bool
	scoreFlag      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "umd",
		Short: "USE Method Daemon - System performance analysis tool",
		Long: `umd implements Brendan Gregg's USE Method for quick system health checks.

The USE Method systematically checks system resources for:
  - Utilization: How busy is the resource?
  - Saturation: How much extra work is queued?
  - Errors: Are there any errors?

This helps identify ~80% of server issues with ~5% of the effort.`,
		Run: runChecks,
	}

	// Add flags
	rootCmd.Flags().StringVarP(&formatFlag, "format", "f", "table", "Output format: table, json, ai, or tsv")
	rootCmd.Flags().StringVarP(&resourceFlag, "resource", "r", "", "Check specific resource: cpu, memory, network, disk, scheduler, tcp, vmem, filesystem")
	rootCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Continuous monitoring mode")
	rootCmd.Flags().IntVarP(&intervalFlag, "interval", "i", 2, "Refresh interval in seconds (with --watch)")
	rootCmd.Flags().Float64Var(&warnUtil, "warn-util", 70.0, "Warning threshold for utilization percentage")
	rootCmd.Flags().Float64Var(&critUtil, "crit-util", 90.0, "Critical threshold for utilization percentage")
	rootCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose debug logging")
	rootCmd.Flags().BoolVar(&crosscheckFlag, "crosscheck", false, "Cross-validate metrics from multiple sources")
	rootCmd.Flags().BoolVar(&pprofFlag, "pprof", false, "Start pprof server on :6060")
	rootCmd.Flags().BoolVar(&traceFlag, "trace", false, "Enable trace logging to stderr")
	rootCmd.Flags().BoolVar(&rawFlag, "raw", false, "Dump raw metrics to stderr")
	rootCmd.Flags().BoolVar(&scoreFlag, "score", false, "Show health score (0-100)")

	// Add subcommands
	rootCmd.AddCommand(newFlamegraphCmd())
	rootCmd.AddCommand(newWorkloadCmd())
	rootCmd.AddCommand(newBaselineCmd())
	rootCmd.AddCommand(newBenchmarkCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func runChecks(cmd *cobra.Command, args []string) {
	// Setup logger
	logger := logrus.New()
	if verboseFlag {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.WarnLevel)
	}

	// Start pprof if requested
	if pprofFlag {
		stop, err := debug.StartPprofServer(":6060")
		if err != nil {
			fmt.Fprintf(os.Stderr, "pprof server failed: %v\n", err)
		} else {
			defer stop()
			fmt.Fprintln(os.Stderr, "pprof server running on http://localhost:6060/debug/pprof/")
		}
	}

	// Setup thresholds
	thresholds := use.Thresholds{
		WarnUtil: warnUtil,
		CritUtil: critUtil,
	}

	// Create checker
	checker := use.NewChecker(thresholds, logger)

	// Get collectors based on resource flag
	collectors := getCollectors(resourceFlag)
	if len(collectors) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no valid collectors found")
		os.Exit(3)
	}

	// Wrap collectors with timing if trace/raw enabled
	var timedCollectors []*debug.TimedCollector
	if traceFlag || rawFlag {
		wrappedCollectors := make([]use.Collector, len(collectors))
		timedCollectors = make([]*debug.TimedCollector, len(collectors))
		for i, c := range collectors {
			tc := debug.NewTimedCollector(c)
			timedCollectors[i] = tc
			wrappedCollectors[i] = tc
		}
		collectors = wrappedCollectors
	}

	// Setup formatter
	format := output.FormatTable
	switch formatFlag {
	case "json":
		format = output.FormatJSON
	case "ai", "llm":
		format = output.FormatAI
	case "tsv":
		format = output.FormatTSV
	}
	formatter := output.NewFormatter(format, os.Stdout)
	formatter.SetShowScore(scoreFlag)

	if watchFlag {
		// Create sparkline tracker for watch mode
		sparklineTracker := output.NewSparklineTracker(20)
		formatter.SetSparklineTracker(sparklineTracker)
		runWatchMode(checker, collectors, formatter, timedCollectors, time.Duration(intervalFlag)*time.Second)
	} else {
		runOnce(checker, collectors, formatter, timedCollectors)
	}
}

func getCollectors(resource string) []use.Collector {
	allCollectors := []use.Collector{
		cpu.New(),
		memory.New(),
		network.New(),
		disk.New(),
		scheduler.New(),
		tcp.New(),
		vmem.New(),
		filesystem.New(),
	}

	if resource == "" {
		return allCollectors
	}

	resource = strings.ToLower(resource)
	for _, c := range allCollectors {
		if strings.ToLower(c.Name()) == resource {
			return []use.Collector{c}
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown resource: %s\n", resource)
	fmt.Fprintln(os.Stderr, "Available resources: cpu, memory, network, disk, scheduler, tcp, vmem, filesystem")
	return nil
}

func runOnce(checker *use.Checker, collectors []use.Collector, formatter *output.Formatter, timedCollectors []*debug.TimedCollector) {
	checks := checker.RunAll(collectors)

	if err := formatter.Render(checks); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering output: %v\n", err)
		os.Exit(1)
	}

	// Debug output to stderr
	printDebugOutput(checks, timedCollectors)

	// Cross-check if enabled
	if crosscheckFlag {
		validations, sanity := crosscheck.RunCrossChecks(checks)
		crosscheck.Report(os.Stderr, validations, sanity)
	}

	// Print drill-down suggestions to stderr if issues exist
	printDrillDown(checks)
}

func runWatchMode(checker *use.Checker, collectors []use.Collector, formatter *output.Formatter, timedCollectors []*debug.TimedCollector, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately first - collect data before clearing screen
	checks := checker.RunAll(collectors)
	clearScreen()
	if err := formatter.Render(checks); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering output: %v\n", err)
	}
	fmt.Printf("\nPress Ctrl+C to exit. Refreshing every %v...\n", interval)

	for range ticker.C {
		// Collect data first, then clear and display
		checks := checker.RunAll(collectors)
		clearScreen()
		if err := formatter.Render(checks); err != nil {
			fmt.Fprintf(os.Stderr, "Error rendering output: %v\n", err)
		}
		fmt.Printf("\nPress Ctrl+C to exit. Refreshing every %v...\n", interval)
	}
}

func printDebugOutput(checks []use.Check, timedCollectors []*debug.TimedCollector) {
	if rawFlag {
		debug.DumpRawMetrics(os.Stderr, checks)
	}

	if traceFlag && len(timedCollectors) > 0 {
		timings := make([]debug.CollectorTiming, len(timedCollectors))
		for i, tc := range timedCollectors {
			timings[i] = tc.Timing
		}
		debug.TimingReport(os.Stderr, timings)
	}
}

func printDrillDown(checks []use.Check) {
	suggestions := output.GetDrillDownSuggestions(checks)
	if len(suggestions) > 0 {
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		cmdStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, titleStyle.Render("Diagnostic Suggestions"))
		fmt.Fprintln(os.Stderr, dimStyle.Render(strings.Repeat("─", 40)))
		for metric, suggs := range suggestions {
			fmt.Fprintf(os.Stderr, "  %s:\n", lipgloss.NewStyle().Bold(true).Render(metric))
			for _, s := range suggs {
				fmt.Fprintf(os.Stderr, "    $ %s  %s\n",
					cmdStyle.Render(s.Command),
					dimStyle.Render("("+s.Reason+")"))
			}
		}
	}
}

func clearScreen() {
	// Move cursor to home position and clear screen
	fmt.Print("\033[H\033[2J\033[3J")
	os.Stdout.Sync()
}
