package reporter

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// JUnitReporter generates JUnit XML reports compatible with Prow/CI systems.
type JUnitReporter struct {
	outputDir string
}

// NewJUnitReporter creates a JUnit XML reporter that writes to the given directory.
func NewJUnitReporter(outputDir string) *JUnitReporter {
	return &JUnitReporter{outputDir: outputDir}
}

func (j *JUnitReporter) StartSuite(name string)                                         {}
func (j *JUnitReporter) StartSpec(name string)                                          {}
func (j *JUnitReporter) StepResult(phase string, stepIndex int, stepText string, o StepOutcome) {}
func (j *JUnitReporter) EndSpec(report *TestReport)                                     {}
func (j *JUnitReporter) EndSuite(report *SuiteReport)                                   {}

// Finalize writes the JUnit XML file.
func (j *JUnitReporter) Finalize(report *SuiteReport) error {
	if j.outputDir == "" {
		return nil
	}

	if err := os.MkdirAll(j.outputDir, 0755); err != nil {
		return fmt.Errorf("creating junit output dir: %w", err)
	}

	suite := junitTestSuite{
		Name:     report.Name,
		Tests:    len(report.Tests),
		Failures: report.Failed,
		Skipped:  report.Skipped,
		Time:     report.Duration.Seconds(),
	}

	for _, tr := range report.Tests {
		tc := junitTestCase{
			Name:      tr.Name,
			ClassName: report.Name,
			Time:      tr.Duration.Seconds(),
		}

		// Build system-out from step details
		var output strings.Builder
		for _, step := range tr.Steps {
			status := "PASS"
			if step.Outcome.Status == StepFailed {
				status = "FAIL"
			}
			output.WriteString(fmt.Sprintf("[%s] %s: %s", status, step.Phase, step.Text))
			if step.Outcome.Output != "" {
				output.WriteString(fmt.Sprintf(" → %s", step.Outcome.Output))
			}
			output.WriteString("\n")

			for _, tc := range step.Outcome.ToolCalls {
				output.WriteString(fmt.Sprintf("  tool: %s (%s)\n", tc.ToolName, tc.Duration))
				if tc.Output != "" {
					output.WriteString(fmt.Sprintf("  output: %s\n", truncateJunit(tc.Output, 500)))
				}
			}
		}
		tc.SystemOut = &junitOutput{Data: output.String()}

		switch tr.Status {
		case TestFailed:
			tc.Failure = &junitFailure{
				Message: tr.Error,
				Data:    output.String(),
			}
		case TestSkipped:
			tc.Skipped = &junitSkipped{
				Message: tr.Error,
			}
		}

		suite.TestCases = append(suite.TestCases, tc)
	}

	data, err := xml.MarshalIndent(junitTestSuites{Suites: []junitTestSuite{suite}}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling junit xml: %w", err)
	}

	filename := filepath.Join(j.outputDir, fmt.Sprintf("junit_%s_%d.xml", sanitizeFilename(report.Name), time.Now().Unix()))
	header := []byte(xml.Header)
	return os.WriteFile(filename, append(header, data...), 0644)
}

// JUnit XML types

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
	SystemOut *junitOutput  `xml:"system-out,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Data    string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr,omitempty"`
}

type junitOutput struct {
	Data string `xml:",chardata"`
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return r.Replace(s)
}

func truncateJunit(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
