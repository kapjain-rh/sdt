package reporter

import (
	"fmt"
	"strings"
	"time"
)

// ConsoleReporter outputs test results to stdout with colored formatting.
type ConsoleReporter struct{}

// NewConsoleReporter creates a new console reporter.
func NewConsoleReporter() *ConsoleReporter {
	return &ConsoleReporter{}
}

func (c *ConsoleReporter) StartSuite(name string) {
	fmt.Printf("\n=== SUITE: %s\n", name)
}

func (c *ConsoleReporter) StartSpec(name string) {
	fmt.Printf("\n=== RUN  %s\n", name)
}

func (c *ConsoleReporter) StepResult(phase string, stepIndex int, stepText string, outcome StepOutcome) {
	statusIcon := statusSymbol(outcome.Status)
	phaseLabel := fmt.Sprintf("  --- %s", strings.ToUpper(phase))

	// Print phase header on first step
	if stepIndex == 0 {
		fmt.Println(phaseLabel)
	}

	durStr := formatDuration(outcome.Duration)
	fmt.Printf("    %s  %s %s\n", statusIcon, stepText, durStr)

	if outcome.Thinking != "" {
		fmt.Printf("    [THINK]  %s\n", truncate(outcome.Thinking, 120))
	}

	if outcome.Error != "" {
		fmt.Printf("    [ERROR]  %s\n", outcome.Error)
	}
}

func (c *ConsoleReporter) EndSpec(report *TestReport) {
	statusLabel := "PASS"
	if report.Status == TestFailed {
		statusLabel = "FAIL"
	} else if report.Status == TestSkipped {
		statusLabel = "SKIP"
	}

	fmt.Printf("--- %s: %s (%s)\n", statusLabel, report.Name, formatDuration(report.Duration))

	if report.Error != "" {
		fmt.Printf("    Error: %s\n", report.Error)
	}

	if report.Diagnosis != "" {
		fmt.Println()
		fmt.Println("  --- AUTO-DEBUG DIAGNOSIS ---")
		for _, line := range strings.Split(report.Diagnosis, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println("  ----------------------------")
	}
}

func (c *ConsoleReporter) EndSuite(report *SuiteReport) {
	fmt.Println()
	fmt.Println("=== SUMMARY")
	fmt.Printf("  Passed:  %d | Failed: %d | Skipped: %d\n", report.Passed, report.Failed, report.Skipped)
	fmt.Printf("  Duration: %s\n", formatDuration(report.Duration))
}

func (c *ConsoleReporter) Finalize(report *SuiteReport) error {
	return nil
}

// statusSymbol returns a visual indicator for step status.
func statusSymbol(s StepStatus) string {
	switch s {
	case StepPassed:
		return "[PASS] "
	case StepFailed:
		return "[FAIL] "
	case StepSkipped:
		return "[SKIP] "
	default:
		return "[????] "
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return ""
	}
	if d < time.Second {
		return fmt.Sprintf("(%dms)", d.Milliseconds())
	}
	return fmt.Sprintf("(%.1fs)", d.Seconds())
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
