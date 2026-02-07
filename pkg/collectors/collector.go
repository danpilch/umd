// Package collectors provides interfaces and implementations for system metric collection.
package collectors

import "github.com/danpilch/umd/pkg/use"

// Collector is the interface that all resource collectors must implement.
type Collector interface {
	// Name returns the name of the resource being collected (e.g., "CPU", "Memory").
	Name() string

	// Collect gathers USE metrics and returns a slice of checks.
	Collect(thresholds use.Thresholds) ([]use.Check, error)
}

// Registry holds all registered collectors.
type Registry struct {
	collectors []Collector
}

// NewRegistry creates a new collector registry.
func NewRegistry() *Registry {
	return &Registry{
		collectors: make([]Collector, 0),
	}
}

// Register adds a collector to the registry.
func (r *Registry) Register(c Collector) {
	r.collectors = append(r.collectors, c)
}

// Collectors returns all registered collectors.
func (r *Registry) Collectors() []Collector {
	return r.collectors
}

// GetByName returns a collector by name, or nil if not found.
func (r *Registry) GetByName(name string) Collector {
	for _, c := range r.collectors {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
