package main

import (
	"fmt"
	"os"

	"github.com/danpilch/umd/pkg/baseline"
	"github.com/danpilch/umd/pkg/collectors/cpu"
	"github.com/danpilch/umd/pkg/collectors/disk"
	"github.com/danpilch/umd/pkg/collectors/filesystem"
	"github.com/danpilch/umd/pkg/collectors/memory"
	"github.com/danpilch/umd/pkg/collectors/network"
	"github.com/danpilch/umd/pkg/collectors/scheduler"
	"github.com/danpilch/umd/pkg/collectors/tcp"
	"github.com/danpilch/umd/pkg/collectors/vmem"
	"github.com/danpilch/umd/pkg/use"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newBaselineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "Manage performance baselines",
		Long:  "Save, compare, and list performance baselines for drift detection.",
	}

	cmd.AddCommand(newBaselineSaveCmd())
	cmd.AddCommand(newBaselineCompareCmd())
	cmd.AddCommand(newBaselineListCmd())

	return cmd
}

func newBaselineSaveCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save current metrics as a baseline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			thresholds := use.DefaultThresholds()
			checker := use.NewChecker(thresholds, logger)

			collectors := getAllCollectors()
			checks := checker.RunAll(collectors)

			b := baseline.NewBaseline(name, checks)
			if err := b.Save(""); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Baseline %q saved (%d checks)\n", name, len(checks))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Baseline name")

	return cmd
}

func newBaselineCompareCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare current metrics against a saved baseline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Load baseline
			b, err := baseline.Load(name, "")
			if err != nil {
				return err
			}

			// Collect current metrics
			logger := logrus.New()
			logger.SetLevel(logrus.WarnLevel)
			thresholds := use.DefaultThresholds()
			checker := use.NewChecker(thresholds, logger)

			collectors := getAllCollectors()
			current := checker.RunAll(collectors)

			// Compare
			comparisons := baseline.Compare(b, current)
			baseline.RenderComparison(os.Stdout, b, comparisons)

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Baseline name to compare against")

	return cmd
}

func newBaselineListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved baselines",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := baseline.List("")
			if err != nil {
				return err
			}

			if len(names) == 0 {
				fmt.Fprintln(os.Stdout, "No baselines saved.")
				fmt.Fprintf(os.Stdout, "Save one with: umd baseline save --name <name>\n")
				return nil
			}

			fmt.Fprintln(os.Stdout, "Saved baselines:")
			for _, n := range names {
				b, err := baseline.Load(n, "")
				if err != nil {
					fmt.Fprintf(os.Stdout, "  %s (error loading)\n", n)
					continue
				}
				fmt.Fprintf(os.Stdout, "  %-20s %s (%d checks)\n",
					n, b.Timestamp.Format("2006-01-02 15:04:05"), len(b.Checks))
			}
			return nil
		},
	}
}

func getAllCollectors() []use.Collector {
	return []use.Collector{
		cpu.New(),
		memory.New(),
		network.New(),
		disk.New(),
		scheduler.New(),
		tcp.New(),
		vmem.New(),
		filesystem.New(),
	}
}

