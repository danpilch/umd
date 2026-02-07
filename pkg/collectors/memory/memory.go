// Package memory provides memory metrics collection for the USE method.
package memory

// Collector gathers memory-related USE metrics.
type Collector struct{}

// New creates a new memory collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "Memory"
}

// Collect gathers memory metrics. Platform-specific implementation in memory_linux.go and memory_darwin.go.
