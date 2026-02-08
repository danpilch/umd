package crosscheck

import (
	"fmt"

	"github.com/danpilch/umd/pkg/use"
)

// SanityResult holds the outcome of a physical constraint check.
type SanityResult struct {
	Check   string
	Passed  bool
	Details string
}

// RunSanityChecks validates all collected metrics against physical constraints.
func RunSanityChecks(checks []use.Check) []SanityResult {
	var results []SanityResult

	for _, c := range checks {
		// Utilization must be in [0, 100] when expressed as percentage
		if c.Type == use.Utilization && c.Status != use.StatusUnknown {
			if c.RawValue < 0 {
				results = append(results, SanityResult{
					Check:   fmt.Sprintf("%s %s", c.Resource, c.Type),
					Passed:  false,
					Details: fmt.Sprintf("negative value: %.2f", c.RawValue),
				})
			} else if c.RawValue > 100 {
				results = append(results, SanityResult{
					Check:   fmt.Sprintf("%s %s", c.Resource, c.Type),
					Passed:  false,
					Details: fmt.Sprintf("utilization exceeds 100%%: %.2f", c.RawValue),
				})
			} else {
				results = append(results, SanityResult{
					Check:   fmt.Sprintf("%s %s", c.Resource, c.Type),
					Passed:  true,
					Details: fmt.Sprintf("%.2f%% within [0, 100]", c.RawValue),
				})
			}
		}

		// All values must be non-negative
		if c.RawValue < 0 && c.Status != use.StatusUnknown {
			results = append(results, SanityResult{
				Check:   fmt.Sprintf("%s %s non-negative", c.Resource, c.Type),
				Passed:  false,
				Details: fmt.Sprintf("negative value: %.2f", c.RawValue),
			})
		}

		// Error counts must be non-negative integers
		if c.Type == use.Errors && c.Status != use.StatusUnknown {
			if c.RawValue < 0 {
				results = append(results, SanityResult{
					Check:   fmt.Sprintf("%s error count", c.Resource),
					Passed:  false,
					Details: fmt.Sprintf("negative error count: %.0f", c.RawValue),
				})
			}
		}
	}

	return results
}
