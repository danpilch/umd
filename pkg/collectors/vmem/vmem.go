// Package vmem provides virtual memory metrics collection for the USE method.
package vmem

// Collector gathers virtual memory USE metrics.
type Collector struct{}

// New creates a new virtual memory collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "VMem"
}
