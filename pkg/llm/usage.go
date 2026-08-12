package llm

import (
	"fmt"
	"strings"
	"sync"
)

// UsageTracker accumulates token usage across multiple LLM requests.
type UsageTracker struct {
	mu               sync.Mutex
	TotalInput       int
	TotalOutput      int
	TotalCacheRead   int
	TotalCacheCreate int
	Requests         int
}

// Add accumulates usage from a single response.
func (u *UsageTracker) Add(usage Usage) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.TotalInput += usage.InputTokens
	u.TotalOutput += usage.OutputTokens
	u.TotalCacheRead += usage.CacheRead
	u.TotalCacheCreate += usage.CacheCreate
	u.Requests++
}

// Total returns the aggregated usage.
func (u *UsageTracker) Total() Usage {
	u.mu.Lock()
	defer u.mu.Unlock()
	return Usage{
		InputTokens:  u.TotalInput,
		OutputTokens: u.TotalOutput,
		CacheRead:    u.TotalCacheRead,
		CacheCreate:  u.TotalCacheCreate,
	}
}

// EstimatedCost returns a rough cost estimate in USD based on model pricing.
func (u *UsageTracker) EstimatedCost(model string) float64 {
	u.mu.Lock()
	defer u.mu.Unlock()

	var inputPer1M, outputPer1M float64
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		inputPer1M, outputPer1M = 15.0, 75.0
	case strings.Contains(m, "sonnet"):
		inputPer1M, outputPer1M = 3.0, 15.0
	case strings.Contains(m, "haiku"):
		inputPer1M, outputPer1M = 0.25, 1.25
	case strings.Contains(m, "gemini"):
		inputPer1M, outputPer1M = 1.25, 10.0
	default:
		return 0
	}

	inputCost := float64(u.TotalInput) / 1_000_000 * inputPer1M
	outputCost := float64(u.TotalOutput) / 1_000_000 * outputPer1M
	return inputCost + outputCost
}

// String returns a human-readable summary.
func (u *UsageTracker) String() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return fmt.Sprintf("%s input + %s output = %s total (%d requests)",
		formatTokens(u.TotalInput), formatTokens(u.TotalOutput),
		formatTokens(u.TotalInput+u.TotalOutput), u.Requests)
}

func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
}
