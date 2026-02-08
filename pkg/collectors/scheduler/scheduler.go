// Package scheduler provides scheduler/run-queue metrics collection for the USE method.
package scheduler

// Collector gathers scheduler-related USE metrics.
type Collector struct{}

// New creates a new scheduler collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "Scheduler"
}
