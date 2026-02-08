//go:build darwin

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
	durSec := int(opts.Duration.Seconds())
	if durSec < 1 {
		durSec = 1
	}

	// Try dtrace first (requires root)
	if _, err := exec.LookPath("dtrace"); err == nil {
		return captureDtrace(ctx, opts, durSec)
	}

	// Fallback to sample command
	if _, err := exec.LookPath("sample"); err == nil && opts.PID > 0 {
		return captureSample(ctx, opts, durSec)
	}

	return nil, fmt.Errorf("no profiling tools available: dtrace requires root, sample requires a PID")
}

func captureDtrace(ctx context.Context, opts CaptureOptions, durSec int) (*CaptureResult, error) {
	probe := fmt.Sprintf("profile-%d", opts.Frequency)
	script := fmt.Sprintf(`%s /pid == %d/ { @[ustack()] = count(); }`, probe, opts.PID)
	if opts.PID == 0 {
		script = fmt.Sprintf(`%s { @[ustack()] = count(); }`, probe)
	}

	cmd := exec.CommandContext(ctx, "dtrace", "-n", script, "-c",
		fmt.Sprintf("sleep %d", durSec))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("dtrace failed: %v (%s)", err, stderr.String())
	}

	var collapsed bytes.Buffer
	CollapseDtrace(&stdout, &collapsed)

	return &CaptureResult{
		CollapsedStacks: collapsed.String(),
		Duration:        time.Duration(durSec) * time.Second,
	}, nil
}

func captureSample(ctx context.Context, opts CaptureOptions, durSec int) (*CaptureResult, error) {
	cmd := exec.CommandContext(ctx, "sample",
		strconv.Itoa(opts.PID),
		strconv.Itoa(durSec))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sample failed: %v", err)
	}

	// Sample output needs custom collapsing - use dtrace collapser as approximation
	var collapsed bytes.Buffer
	CollapseDtrace(&stdout, &collapsed)

	return &CaptureResult{
		CollapsedStacks: collapsed.String(),
		Duration:        time.Duration(durSec) * time.Second,
	}, nil
}
