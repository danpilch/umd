//go:build linux

package flamegraph

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

func platformCapture(ctx context.Context, opts CaptureOptions) (*CaptureResult, error) {
	// Check if perf is available
	if _, err := exec.LookPath("perf"); err != nil {
		return nil, fmt.Errorf("perf not found: install linux-tools-common or equivalent")
	}

	durSec := int(opts.Duration.Seconds())
	if durSec < 1 {
		durSec = 1
	}

	// Run perf record
	args := []string{
		"record", "-F", strconv.Itoa(opts.Frequency),
		"-a", "-g", "--", "sleep", strconv.Itoa(durSec),
	}
	if opts.PID > 0 {
		args = []string{
			"record", "-F", strconv.Itoa(opts.Frequency),
			"-p", strconv.Itoa(opts.PID), "-g",
			"--", "sleep", strconv.Itoa(durSec),
		}
	}

	cmd := exec.CommandContext(ctx, "perf", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("perf record failed: %v (%s)", err, stderr.String())
	}

	// Run perf script to get stack traces
	scriptCmd := exec.CommandContext(ctx, "perf", "script")
	var scriptOut bytes.Buffer
	scriptCmd.Stdout = &scriptOut
	if err := scriptCmd.Run(); err != nil {
		return nil, fmt.Errorf("perf script failed: %v", err)
	}

	// Collapse perf output
	var collapsed bytes.Buffer
	CollapsePerf(&scriptOut, &collapsed)

	return &CaptureResult{
		CollapsedStacks: collapsed.String(),
		Duration:        time.Duration(durSec) * time.Second,
	}, nil
}
