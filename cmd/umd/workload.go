package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danpilch/umd/pkg/workload"
	"github.com/spf13/cobra"
)

func newWorkloadCmd() *cobra.Command {
	var (
		formatStr string
		topN      int
	)

	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Characterize system workload - what is the system doing?",
		Long: `Analyzes running processes, load averages, and process states to answer
"what is the system actually doing?" Provides top CPU/memory consumers
and load trend analysis.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := workload.Characterize()
			if err != nil {
				return fmt.Errorf("workload characterization failed: %w", err)
			}

			switch formatStr {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			default:
				report.Render(os.Stdout, topN)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&formatStr, "format", "f", "text", "Output format: text or json")
	cmd.Flags().IntVarP(&topN, "top", "n", 10, "Number of top processes to show")

	return cmd
}
