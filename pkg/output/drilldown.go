package output

import (
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Suggestion represents a diagnostic next-step.
type Suggestion struct {
	Tool    string
	Command string
	Reason  string
}

// DrillDown returns diagnostic suggestions for a check with issues.
func DrillDown(check use.Check) []Suggestion {
	resource := strings.ToLower(check.Resource)

	var suggestions []Suggestion

	switch {
	case strings.Contains(resource, "cpu"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"top", "top -o cpu", "Identify top CPU consumers"},
				Suggestion{"umd", "umd workload --top 10", "Full workload characterization"},
			)
			if check.Type == use.Utilization {
				suggestions = append(suggestions,
					Suggestion{"umd", "umd flamegraph -d 10", "Capture CPU flame graph"},
				)
			}
		}

	case strings.Contains(resource, "memory"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"top", "top -o mem", "Identify top memory consumers"},
				Suggestion{"umd", "umd workload --top 10", "Full workload characterization"},
			)
			if check.Type == use.Saturation {
				suggestions = append(suggestions,
					Suggestion{"vmstat", "vmstat 1 5", "Monitor swap activity"},
				)
			}
		}

	case strings.Contains(resource, "disk"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"df", "df -ih", "Check inode and disk usage"},
			)
			if check.Type == use.Utilization {
				suggestions = append(suggestions,
					Suggestion{"iostat", "iostat -x 1 3", "Detailed I/O statistics"},
				)
			}
		}

	case strings.Contains(resource, "filesystem"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"df", "df -ih", "Check inode and disk usage"},
			)
		}

	case strings.Contains(resource, "network"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"netstat", "netstat -s", "Network statistics summary"},
			)
		}

	case strings.Contains(resource, "tcp"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"ss", "ss -s", "Socket statistics summary"},
				Suggestion{"netstat", "netstat -s -p tcp", "TCP statistics"},
			)
		}

	case strings.Contains(resource, "scheduler"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"umd", "umd workload", "Workload characterization"},
			)
		}

	case strings.Contains(resource, "vmem"):
		if check.Status == use.StatusError || check.Status == use.StatusWarning {
			suggestions = append(suggestions,
				Suggestion{"vmstat", "vmstat 1 5", "Virtual memory statistics"},
			)
		}
	}

	return suggestions
}

// GetDrillDownSuggestions returns all suggestions for checks with issues.
func GetDrillDownSuggestions(checks []use.Check) map[string][]Suggestion {
	results := make(map[string][]Suggestion)
	for _, c := range checks {
		if c.Status == use.StatusWarning || c.Status == use.StatusError {
			key := c.Resource + " " + string(c.Type)
			suggestions := DrillDown(c)
			if len(suggestions) > 0 {
				results[key] = suggestions
			}
		}
	}
	return results
}
