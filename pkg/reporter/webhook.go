package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookReporter sends suite results to a webhook URL (Slack-compatible).
type WebhookReporter struct {
	url     string
	channel string
}

// NewWebhookReporter creates a reporter that POSTs results to the given URL.
// For Slack, pass the incoming webhook URL. The channel parameter is optional.
func NewWebhookReporter(url string, channel string) *WebhookReporter {
	return &WebhookReporter{url: url, channel: channel}
}

func (w *WebhookReporter) StartSuite(name string)                                                 {}
func (w *WebhookReporter) StartSpec(name string)                                                  {}
func (w *WebhookReporter) StepResult(phase string, stepIndex int, stepText string, o StepOutcome) {}
func (w *WebhookReporter) EndSpec(report *TestReport)                                             {}
func (w *WebhookReporter) EndSuite(report *SuiteReport)                                           {}

func (w *WebhookReporter) Finalize(report *SuiteReport) error {
	if w.url == "" {
		return nil
	}

	icon := ":white_check_mark:"
	if report.Failed > 0 {
		icon = ":x:"
	}

	text := fmt.Sprintf("%s *SDT Suite: %s*\n", icon, report.Name)
	text += fmt.Sprintf("Passed: %d | Failed: %d | Skipped: %d | Duration: %s\n",
		report.Passed, report.Failed, report.Skipped, formatWebhookDuration(report.Duration))

	if report.Failed > 0 {
		text += "\n*Failed specs:*\n"
		for _, t := range report.Tests {
			if t.Status == TestFailed {
				text += fmt.Sprintf("  - %s: %s\n", t.Name, truncateWebhook(t.Error, 100))
			}
		}
	}

	flakyCount := 0
	for _, t := range report.Tests {
		if t.Flaky {
			flakyCount++
		}
	}
	if flakyCount > 0 {
		text += fmt.Sprintf("\n:warning: %d flaky spec(s) passed on retry\n", flakyCount)
	}

	if report.TokenUsage != nil && report.TokenUsage.TotalTokens > 0 {
		text += fmt.Sprintf("\nTokens: %d (~$%.2f)\n", report.TokenUsage.TotalTokens, report.TokenUsage.EstimatedCost)
	}

	payload := map[string]string{"text": text}
	if w.channel != "" {
		payload["channel"] = w.channel
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(w.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func formatWebhookDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

func truncateWebhook(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
