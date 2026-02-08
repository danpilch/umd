package output

import "github.com/danpilch/umd/pkg/use"

// HealthScore computes a 0-100 health score from check results.
// Starts at 100, -15 per critical/error, -5 per warning, -3 per unknown.
func HealthScore(checks []use.Check) int {
	score := 100
	for _, c := range checks {
		switch c.Status {
		case use.StatusError:
			score -= 15
		case use.StatusWarning:
			score -= 5
		case use.StatusUnknown:
			score -= 3
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// ScoreLabel returns a human-readable label for a health score.
func ScoreLabel(score int) string {
	if score >= 80 {
		return "Healthy"
	}
	if score >= 50 {
		return "Degraded"
	}
	return "Critical"
}
