// Package crosscheck provides cross-validation of metrics from multiple sources.
package crosscheck

import (
	"math"
	"sort"
)

// ValidationStatus indicates the confidence level of a cross-checked metric.
type ValidationStatus string

const (
	StatusValid    ValidationStatus = "valid"
	StatusSuspect  ValidationStatus = "suspect"
	StatusConflict ValidationStatus = "conflict"
)

// Source represents a single metric reading from a specific source.
type Source struct {
	Name    string
	Value   float64
	Unit    string
	RawData string
}

// ValidationResult holds the cross-check outcome for a metric.
type ValidationResult struct {
	Metric       string
	Sources      []Source
	Consensus    float64
	MaxDeviation float64
	Status       ValidationStatus
}

// Validator cross-checks metrics from multiple sources.
type Validator struct {
	SuspectThreshold  float64 // deviation % to mark suspect (default 5%)
	ConflictThreshold float64 // deviation % to mark conflict (default 20%)
}

// NewValidator creates a validator with default thresholds.
func NewValidator() *Validator {
	return &Validator{
		SuspectThreshold:  5.0,
		ConflictThreshold: 20.0,
	}
}

// CrossCheck validates a metric by comparing values from multiple sources.
// Returns a ValidationResult with consensus (median) and deviation analysis.
func (v *Validator) CrossCheck(metric string, sources []Source) ValidationResult {
	result := ValidationResult{
		Metric:  metric,
		Sources: sources,
		Status:  StatusValid,
	}

	if len(sources) == 0 {
		return result
	}

	if len(sources) == 1 {
		result.Consensus = sources[0].Value
		return result
	}

	// Calculate consensus via median
	values := make([]float64, len(sources))
	for i, s := range sources {
		values[i] = s.Value
	}
	sort.Float64s(values)

	if len(values)%2 == 0 {
		result.Consensus = (values[len(values)/2-1] + values[len(values)/2]) / 2
	} else {
		result.Consensus = values[len(values)/2]
	}

	// Calculate max deviation from consensus
	for _, val := range values {
		if result.Consensus == 0 {
			if val != 0 {
				result.MaxDeviation = 100.0
			}
			continue
		}
		dev := math.Abs(val-result.Consensus) / result.Consensus * 100
		if dev > result.MaxDeviation {
			result.MaxDeviation = dev
		}
	}

	// Evaluate status
	if result.MaxDeviation >= v.ConflictThreshold {
		result.Status = StatusConflict
	} else if result.MaxDeviation >= v.SuspectThreshold {
		result.Status = StatusSuspect
	}

	return result
}
