// Package filesystem provides filesystem inode and file descriptor metrics for the USE method.
package filesystem

// Collector gathers filesystem-level USE metrics (inodes, FDs).
type Collector struct{}

// New creates a new filesystem collector.
func New() *Collector {
	return &Collector{}
}

// Name returns the collector name.
func (c *Collector) Name() string {
	return "Filesystem"
}
