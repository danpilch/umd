// Package use provides types and utilities for the USE Method system analysis.
package use

// MetricType represents the type of USE metric being measured.
type MetricType string

const (
	Utilization MetricType = "utilization"
	Saturation  MetricType = "saturation"
	Errors      MetricType = "errors"
)

// Status represents the health status of a check.
type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
	StatusUnknown Status = "unknown"
)

// Check represents a single USE method check result.
type Check struct {
	Resource    string     `json:"resource"`
	Type        MetricType `json:"type"`
	Value       string     `json:"value"`
	RawValue    float64    `json:"raw_value"`
	Status      Status     `json:"status"`
	Description string     `json:"description"`
	Command     string     `json:"command"`
}

// Thresholds defines warning and critical thresholds for utilization metrics.
type Thresholds struct {
	WarnUtil float64
	CritUtil float64
}

// DefaultThresholds returns the default threshold values.
func DefaultThresholds() Thresholds {
	return Thresholds{
		WarnUtil: 70.0,
		CritUtil: 90.0,
	}
}

// EvaluateUtilization returns the appropriate status based on utilization percentage.
func (t Thresholds) EvaluateUtilization(percent float64) Status {
	if percent >= t.CritUtil {
		return StatusError
	}
	if percent >= t.WarnUtil {
		return StatusWarning
	}
	return StatusOK
}

// EvaluateErrors returns status based on error count.
func EvaluateErrors(count int64) Status {
	if count > 0 {
		return StatusWarning
	}
	return StatusOK
}

// EvaluateSaturation returns status based on saturation value and threshold.
func EvaluateSaturation(value, threshold float64) Status {
	if value > threshold {
		return StatusWarning
	}
	return StatusOK
}
