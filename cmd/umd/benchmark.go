package main

import (
	"fmt"
	"os"

	"github.com/danpilch/umd/pkg/benchmark"
	"github.com/danpilch/umd/pkg/use"
	"github.com/spf13/cobra"
)

func newBenchmarkCmd() *cobra.Command {
	var iterations int

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Self-benchmark collectors to measure tool overhead",
		Long: `Runs each collector multiple times to measure latency percentiles,
value stability (standard deviation), and overall tool overhead.
Validates that the measurement tool isn't perturbing what it measures.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			thresholds := use.DefaultThresholds()
			collectors := getAllCollectors()

			opts := benchmark.DefaultOptions()
			if iterations > 0 {
				opts.Iterations = iterations
			}

			fmt.Fprintf(os.Stderr, "Benchmarking %d collectors (%d iterations, %d warmup)...\n",
				len(collectors), opts.Iterations, opts.Warmup)

			results := benchmark.Run(collectors, thresholds, opts)
			overhead := benchmark.MeasureOverhead()
			benchmark.RenderResults(os.Stdout, results, overhead)

			return nil
		},
	}

	cmd.Flags().IntVarP(&iterations, "iterations", "n", 20, "Number of iterations per collector")

	return cmd
}
