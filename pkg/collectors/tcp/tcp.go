// Package tcp provides TCP/IP stack metrics collection for the USE method.
package tcp

// Collector gathers TCP/IP stack USE metrics.
type Collector struct{}

// New creates a new TCP collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "TCP"
}
