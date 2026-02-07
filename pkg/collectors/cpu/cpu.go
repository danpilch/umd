// Package cpu provides CPU metrics collection for the USE method.
package cpu

// Collector gathers CPU-related USE metrics.
type Collector struct{}

// New creates a new CPU collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "CPU"
}

// Collect gathers CPU metrics. Platform-specific implementation in cpu_linux.go and cpu_darwin.go.
// The Collect method is implemented in platform-specific files.
