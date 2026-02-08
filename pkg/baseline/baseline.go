// Package baseline provides performance baseline save/load and drift detection.
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danpilch/umd/pkg/use"
)

// Baseline represents a snapshot of system performance metrics.
type Baseline struct {
	Name      string      `json:"name"`
	Timestamp time.Time   `json:"timestamp"`
	Hostname  string      `json:"hostname"`
	Checks    []use.Check `json:"checks"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// DefaultDir returns the default baseline storage directory.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".umd/baselines"
	}
	return filepath.Join(home, ".umd", "baselines")
}

// Save writes a baseline to a JSON file.
func (b *Baseline) Save(dir string) error {
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create baseline directory: %w", err)
	}

	path := filepath.Join(dir, b.Name+".json")
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal baseline: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write baseline: %w", err)
	}
	return nil
}

// Load reads a baseline from a JSON file.
func Load(name, dir string) (*Baseline, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	path := filepath.Join(dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read baseline %q: %w", name, err)
	}

	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("cannot parse baseline: %w", err)
	}
	return &b, nil
}

// List returns all saved baseline names.
func List(dir string) ([]string, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5])
		}
	}
	return names, nil
}

// NewBaseline creates a new baseline from current checks.
func NewBaseline(name string, checks []use.Check) *Baseline {
	hostname, _ := os.Hostname()
	return &Baseline{
		Name:      name,
		Timestamp: time.Now(),
		Hostname:  hostname,
		Checks:    checks,
	}
}
