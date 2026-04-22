package tcms

import (
	"strconv"

	"github.com/sdt-project/sdt/pkg/log"
	"github.com/sdt-project/sdt/pkg/reporter"
)

// TCMSReporter implements the reporter.Reporter interface and reports
// results to Kiwi TCMS in real-time as tests complete.
type TCMSReporter struct {
	client     *KiwiClient
	runID      int
	buildID    int
	executions map[string]int // CaseID string -> execution ID
}

// NewTCMSReporter creates a TCMS reporter for the given test run.
func NewTCMSReporter(client *KiwiClient, runID, buildID int) *TCMSReporter {
	return &TCMSReporter{
		client:     client,
		runID:      runID,
		buildID:    buildID,
		executions: make(map[string]int),
	}
}

func (t *TCMSReporter) StartSuite(name string) {
	log.Infof("TCMS", "Suite %q started — reporting to run %d", name, t.runID)
}

func (t *TCMSReporter) StartSpec(name string) {}

func (t *TCMSReporter) StepResult(phase string, stepIndex int, stepText string, outcome reporter.StepOutcome) {
}

// EndSpec reports the test result to Kiwi TCMS.
func (t *TCMSReporter) EndSpec(report *reporter.TestReport) {
	if report.CaseID == "" {
		return
	}

	caseID, err := strconv.Atoi(report.CaseID)
	if err != nil {
		log.Warnf("TCMS", "Invalid CaseID %q, skipping TCMS reporting", report.CaseID)
		return
	}

	// Find or create execution for this case in the run
	execID, ok := t.executions[report.CaseID]
	if !ok {
		// Look for existing execution
		execs, err := t.client.FilterTestExecutions(map[string]interface{}{
			"run":  t.runID,
			"case": caseID,
		})
		if err == nil && len(execs) > 0 {
			execID = execs[0].ID
		} else {
			// Create new execution
			execID, err = t.client.CreateTestExecution(t.runID, caseID, t.buildID)
			if err != nil {
				log.Errorf("TCMS", "Failed to create execution for case %d: %v", caseID, err)
				return
			}
		}
		t.executions[report.CaseID] = execID
	}

	if err := t.client.ReportResult(execID, report); err != nil {
		log.Errorf("TCMS", "Failed to report result for case %s: %v", report.CaseID, err)
	}
}

func (t *TCMSReporter) EndSuite(report *reporter.SuiteReport) {}

// Finalize completes the test run in Kiwi TCMS.
func (t *TCMSReporter) Finalize(report *reporter.SuiteReport) error {
	log.Infof("TCMS", "Run %d finalized: %d passed, %d failed, %d skipped",
		t.runID, report.Passed, report.Failed, report.Skipped)
	return nil
}
