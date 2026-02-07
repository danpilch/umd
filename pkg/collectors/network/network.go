// Package network provides network interface metrics collection for the USE method.
package network

// Collector gathers network-related USE metrics.
type Collector struct{}

// New creates a new network collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "Network"
}

// Collect gathers network metrics. Platform-specific implementation in network_linux.go and network_darwin.go.
