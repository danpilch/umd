package output

import (
	"strings"
	"sync"
)

// SparklineTracker keeps a rolling window of metric values for sparkline rendering.
type SparklineTracker struct {
	mu     sync.Mutex
	data   map[string][]float64
	maxLen int
}

// NewSparklineTracker creates a tracker with a fixed window size.
func NewSparklineTracker(maxLen int) *SparklineTracker {
	if maxLen < 1 {
		maxLen = 20
	}
	return &SparklineTracker{
		data:   make(map[string][]float64),
		maxLen: maxLen,
	}
}

// Record adds a new value for a metric key.
func (s *SparklineTracker) Record(key string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = append(s.data[key], value)
	if len(s.data[key]) > s.maxLen {
		s.data[key] = s.data[key][len(s.data[key])-s.maxLen:]
	}
}

// Sparkline returns a Unicode sparkline string for a metric key.
func (s *SparklineTracker) Sparkline(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	values, ok := s.data[key]
	if !ok || len(values) == 0 {
		return ""
	}

	return renderSparkline(values)
}

// sparkline block characters from lowest to highest
var sparkBlocks = []rune{
	'\u2581', // ▁
	'\u2582', // ▂
	'\u2583', // ▃
	'\u2584', // ▄
	'\u2585', // ▅
	'\u2586', // ▆
	'\u2587', // ▇
	'\u2588', // █
}

func renderSparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}

	// Find min and max
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	var b strings.Builder
	rng := max - min
	for _, v := range values {
		idx := 0
		if rng > 0 {
			idx = int((v - min) / rng * float64(len(sparkBlocks)-1))
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(sparkBlocks[idx])
	}

	return b.String()
}
