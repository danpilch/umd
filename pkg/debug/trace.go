package debug

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// TraceLogger provides step-by-step trace logging for collector operations.
type TraceLogger struct {
	mu      sync.Mutex
	writer  io.Writer
	enabled bool
}

// NewTraceLogger creates a trace logger writing to the given writer.
func NewTraceLogger(w io.Writer) *TraceLogger {
	return &TraceLogger{
		writer:  w,
		enabled: true,
	}
}

// Log records a trace entry for a collector step.
func (t *TraceLogger) Log(collector, step, detail string) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.writer, "[TRACE %s] %s: %s - %s\n",
		time.Now().Format("15:04:05.000"), collector, step, detail)
}

// LogValue records a raw value reading from a specific source.
func (t *TraceLogger) LogValue(collector, source, rawStr string, parsed float64) {
	if !t.enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.writer, "[TRACE %s] %s: source=%s raw=%q parsed=%.4f\n",
		time.Now().Format("15:04:05.000"), collector, source, rawStr, parsed)
}

// defaultTraceWriter returns stderr for trace output.
func defaultTraceWriter() io.Writer {
	return os.Stderr
}
