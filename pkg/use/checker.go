package use

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Checker orchestrates the collection of USE metrics from multiple collectors.
type Checker struct {
	thresholds Thresholds
	logger     *logrus.Logger
}

// Collector interface for resource collectors.
type Collector interface {
	Name() string
	Collect(thresholds Thresholds) ([]Check, error)
}

// NewChecker creates a new USE method checker.
func NewChecker(thresholds Thresholds, logger *logrus.Logger) *Checker {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}
	return &Checker{
		thresholds: thresholds,
		logger:     logger,
	}
}

// RunAll executes all collectors and returns aggregated results.
func (c *Checker) RunAll(collectors []Collector) []Check {
	var (
		allChecks []Check
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	for _, collector := range collectors {
		wg.Add(1)
		go func(col Collector) {
			defer wg.Done()

			c.logger.WithField("collector", col.Name()).Debug("Running collector")

			checks, err := col.Collect(c.thresholds)
			if err != nil {
				c.logger.WithFields(logrus.Fields{
					"collector": col.Name(),
					"error":     err,
				}).Warn("Collector failed")

				// Add unknown status check for failed collector
				checks = []Check{{
					Resource:    col.Name(),
					Type:        Utilization,
					Value:       "unknown",
					RawValue:    0,
					Status:      StatusUnknown,
					Description: err.Error(),
				}}
			}

			mu.Lock()
			allChecks = append(allChecks, checks...)
			mu.Unlock()
		}(collector)
	}

	wg.Wait()
	return allChecks
}

// RunOne executes a single collector by name.
func (c *Checker) RunOne(collector Collector) ([]Check, error) {
	c.logger.WithField("collector", collector.Name()).Debug("Running collector")
	return collector.Collect(c.thresholds)
}

// Summary calculates summary statistics from check results.
type Summary struct {
	Total    int
	OK       int
	Warnings int
	Errors   int
	Unknown  int
}

// Summarize calculates summary statistics from check results.
func Summarize(checks []Check) Summary {
	s := Summary{Total: len(checks)}
	for _, check := range checks {
		switch check.Status {
		case StatusOK:
			s.OK++
		case StatusWarning:
			s.Warnings++
		case StatusError:
			s.Errors++
		case StatusUnknown:
			s.Unknown++
		}
	}
	return s
}

// ExitCode returns the appropriate exit code based on check results.
func ExitCode(checks []Check) int {
	summary := Summarize(checks)
	if summary.Unknown > 0 && summary.Errors == 0 && summary.Warnings == 0 {
		return 3 // Tool error
	}
	if summary.Errors > 0 {
		return 2 // Critical issues
	}
	if summary.Warnings > 0 {
		return 1 // Warnings
	}
	return 0 // All OK
}
