// Package flamegraph provides CPU profiling and SVG flame graph generation.
package flamegraph

import (
	"context"
	"time"
)

// CaptureOptions configures a flame graph capture session.
type CaptureOptions struct {
	Duration  time.Duration
	Frequency int    // sampling frequency in Hz
	PID       int    // 0 = system-wide
	Output    string // output file path
}

// DefaultCaptureOptions returns sensible defaults.
func DefaultCaptureOptions() CaptureOptions {
	return CaptureOptions{
		Duration:  10 * time.Second,
		Frequency: 99,
		Output:    "flamegraph.svg",
	}
}

// CaptureResult holds the result of a capture.
type CaptureResult struct {
	CollapsedStacks string // folded stack format
	SVGPath         string
	SampleCount     int
	Duration        time.Duration
}

// Capture runs a profiling capture and returns collapsed stacks.
// Platform-specific implementation in capture_linux.go and capture_darwin.go.
func Capture(ctx context.Context, opts CaptureOptions) (*CaptureResult, error) {
	return platformCapture(ctx, opts)
}
