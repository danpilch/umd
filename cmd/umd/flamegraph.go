package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danpilch/umd/pkg/flamegraph"
	"github.com/spf13/cobra"
)

func newFlamegraphCmd() *cobra.Command {
	var (
		duration  int
		frequency int
		pid       int
		output    string
		svgOnly   bool
	)

	cmd := &cobra.Command{
		Use:   "flamegraph",
		Short: "Capture CPU profile and generate flame graph SVG",
		Long: `Captures CPU stack traces using platform profiling tools (perf on Linux,
dtrace/sample on macOS) and generates an SVG flame graph.

Requires elevated privileges for system-wide capture.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := flamegraph.CaptureOptions{
				Duration:  time.Duration(duration) * time.Second,
				Frequency: frequency,
				PID:       pid,
				Output:    output,
			}

			fmt.Fprintf(os.Stderr, "Capturing %ds profile at %dHz...\n", duration, frequency)

			ctx, cancel := context.WithTimeout(context.Background(),
				opts.Duration+30*time.Second)
			defer cancel()

			result, err := flamegraph.Capture(ctx, opts)
			if err != nil {
				return fmt.Errorf("capture failed: %w", err)
			}

			if result.CollapsedStacks == "" {
				return fmt.Errorf("no stack samples captured")
			}

			// Generate SVG
			svgOpts := flamegraph.DefaultSVGOptions()
			svgOpts.Title = fmt.Sprintf("Flame Graph (%ds @ %dHz)", duration, frequency)

			svgFile, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("cannot create output file: %w", err)
			}
			defer svgFile.Close()

			if err := flamegraph.GenerateSVG(
				strings.NewReader(result.CollapsedStacks),
				svgFile, svgOpts); err != nil {
				return fmt.Errorf("SVG generation failed: %w", err)
			}

			if !svgOnly {
				fmt.Fprintf(os.Stderr, "Flame graph written to %s\n", output)
				fmt.Fprintf(os.Stderr, "Capture duration: %v\n", result.Duration)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&duration, "duration", "d", 10, "Capture duration in seconds")
	cmd.Flags().IntVarP(&frequency, "frequency", "F", 99, "Sampling frequency in Hz")
	cmd.Flags().IntVarP(&pid, "pid", "p", 0, "Process ID (0 = system-wide)")
	cmd.Flags().StringVarP(&output, "output", "o", "flamegraph.svg", "Output SVG file")
	cmd.Flags().BoolVar(&svgOnly, "svg", false, "Suppress info messages")

	return cmd
}
