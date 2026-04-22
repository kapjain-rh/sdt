package tcms

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/sdt-project/sdt/pkg/log"
	"github.com/sdt-project/sdt/pkg/spec"
)

// SyncSpecs syncs all specs in a suite to Kiwi TCMS as test cases,
// creates a test plan, and adds all cases to it.
func SyncSpecs(client *KiwiClient, suite *spec.Suite, productName string) (*SyncResult, error) {
	productID, err := client.GetOrCreateProduct(productName)
	if err != nil {
		return nil, fmt.Errorf("resolving product %q: %w", productName, err)
	}

	versionID, err := client.GetVersionForProduct(productID)
	if err != nil {
		return nil, fmt.Errorf("resolving version: %w", err)
	}

	categoryID, err := client.GetCategory(productID)
	if err != nil {
		return nil, err
	}

	planTypeID, err := client.GetPlanType()
	if err != nil {
		return nil, fmt.Errorf("resolving plan type: %w", err)
	}

	// Determine plan name from suite
	suiteName := "default"
	if suite.SuiteSpec != nil {
		suiteName = suite.SuiteSpec.Name
	}
	planName := fmt.Sprintf("SDT - %s", suiteName)

	planID, err := client.GetOrCreateTestPlan(planName, productID, versionID, planTypeID)
	if err != nil {
		return nil, fmt.Errorf("creating test plan: %w", err)
	}
	log.Infof("TCMS", "Using test plan %q (id=%d)", planName, planID)

	result := &SyncResult{PlanID: planID}

	for _, testSpec := range suite.Tests {
		if testSpec.IsDraft() {
			log.Debugf("TCMS", "Skipping draft spec %q", testSpec.TestName())
			continue
		}

		priorityID, _ := client.GetPriority(testSpec.Metadata.Priority)
		if priorityID == 0 {
			priorityID, _ = client.GetPriority("")
		}

		caseID, err := client.SyncTestCase(testSpec, productID, categoryID, priorityID)
		if err != nil {
			result.Errors = append(result.Errors, SyncError{
				SpecName: testSpec.TestName(),
				Error:    err.Error(),
			})
			log.Errorf("TCMS", "Failed to sync %q: %v", testSpec.TestName(), err)
			continue
		}

		// Add case to the test plan
		if err := client.AddCaseToPlan(planID, caseID); err != nil {
			log.Debugf("TCMS", "Case %d may already be in plan %d: %v", caseID, planID, err)
		}

		// Update spec file with the Kiwi TCMS CaseID
		if testSpec.FilePath != "" {
			if err := updateSpecCaseID(testSpec.FilePath, caseID); err != nil {
				log.Warnf("TCMS", "Could not update CaseID in %s: %v", testSpec.FilePath, err)
			}
		}

		result.Synced = append(result.Synced, SyncedCase{
			SpecName: testSpec.TestName(),
			CaseID:   caseID,
			FilePath: testSpec.FilePath,
		})
	}

	log.Infof("TCMS", "Sync complete: %d synced, %d errors, plan id=%d", len(result.Synced), len(result.Errors), planID)
	return result, nil
}

// SetupTestRun creates a test run in Kiwi TCMS with all the specs as executions.
// Returns the run ID and a map of CaseID -> execution ID.
func SetupTestRun(client *KiwiClient, suite *spec.Suite, productName, buildName string) (int, int, map[string]int, error) {
	productID, err := client.GetOrCreateProduct(productName)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("resolving product: %w", err)
	}

	versionID, err := client.GetVersionForProduct(productID)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("resolving version: %w", err)
	}

	buildID, err := client.GetOrCreateBuild(versionID, buildName)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("resolving build: %w", err)
	}

	planTypeID, err := client.GetPlanType()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("resolving plan type: %w", err)
	}

	// Get or create test plan
	suiteName := "default"
	if suite.SuiteSpec != nil {
		suiteName = suite.SuiteSpec.Name
	}
	planName := fmt.Sprintf("SDT - %s", suiteName)

	planID, err := client.GetOrCreateTestPlan(planName, productID, versionID, planTypeID)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("resolving test plan: %w", err)
	}

	// Sync test cases and add to plan
	categoryID, err := client.GetCategory(productID)
	if err != nil {
		return 0, 0, nil, err
	}

	caseIDs := make(map[string]int) // spec CaseID string -> TCMS case ID
	for _, testSpec := range suite.Tests {
		if testSpec.Metadata.CaseID == "" {
			continue
		}
		caseID, err := strconv.Atoi(testSpec.Metadata.CaseID)
		if err != nil {
			// Sync to create it
			priorityID, _ := client.GetPriority(testSpec.Metadata.Priority)
			if priorityID == 0 {
				priorityID, _ = client.GetPriority("")
			}
			caseID, err = client.SyncTestCase(testSpec, productID, categoryID, priorityID)
			if err != nil {
				log.Warnf("TCMS", "Failed to sync case for %q: %v", testSpec.TestName(), err)
				continue
			}
		}
		caseIDs[testSpec.Metadata.CaseID] = caseID
		_ = client.AddCaseToPlan(planID, caseID) // Ignore error if already added
	}

	// Get current user for manager
	managerID, err := client.GetCurrentUserID()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("getting current user: %w", err)
	}

	// Create test run
	summary := fmt.Sprintf("SDT Run - %s - %s", suiteName, time.Now().Format("2006-01-02 15:04"))
	runID, err := client.CreateTestRun(summary, planID, buildID, managerID)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("creating test run: %w", err)
	}

	// Create test executions for each case
	execMap := make(map[string]int) // spec CaseID string -> execution ID
	for specCaseID, tcmsCaseID := range caseIDs {
		execID, err := client.CreateTestExecution(runID, tcmsCaseID, buildID)
		if err != nil {
			log.Warnf("TCMS", "Failed to create execution for case %d: %v", tcmsCaseID, err)
			continue
		}
		execMap[specCaseID] = execID
	}

	log.Infof("TCMS", "Test run %d created with %d executions", runID, len(execMap))
	return runID, buildID, execMap, nil
}

// SetupTestRunForPlan creates a test run under an existing TCMS test plan.
// It fetches the plan to determine the product and version, fetches the plan's
// test cases, matches them to local specs by CaseID, and creates a run with
// executions for matched cases. Returns the run ID, build ID, matched case IDs
// (spec CaseID string -> TCMS case ID), and the plan info.
func SetupTestRunForPlan(client *KiwiClient, planID int, buildName string, suite *spec.Suite) (int, int, map[string]int, *TestPlanInfo, error) {
	plan, err := client.GetTestPlan(planID)
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("fetching test plan %d: %w", planID, err)
	}
	log.Infof("TCMS", "Using test plan %q (id=%d, product=%d, version=%d)",
		plan.Name, plan.ID, plan.Product, plan.ProductVersion)

	buildID, err := client.GetOrCreateBuild(plan.ProductVersion, buildName)
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("resolving build: %w", err)
	}

	planCases, err := client.GetTestCasesForPlan(planID)
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("fetching test cases for plan %d: %w", planID, err)
	}
	if len(planCases) == 0 {
		return 0, 0, nil, nil, fmt.Errorf("no test cases found in plan %d", planID)
	}

	planCaseSet := make(map[int]bool, len(planCases))
	for _, tc := range planCases {
		planCaseSet[tc.ID] = true
	}

	matchedCaseIDs := make(map[string]int)
	for _, testSpec := range suite.Tests {
		if testSpec.Metadata.CaseID == "" {
			continue
		}
		caseID, err := strconv.Atoi(testSpec.Metadata.CaseID)
		if err != nil {
			log.Warnf("TCMS", "Spec %q has non-numeric CaseID %q, skipping",
				testSpec.TestName(), testSpec.Metadata.CaseID)
			continue
		}
		if planCaseSet[caseID] {
			matchedCaseIDs[testSpec.Metadata.CaseID] = caseID
		}
	}

	if len(matchedCaseIDs) == 0 {
		return 0, 0, nil, nil, fmt.Errorf(
			"no local specs match any of the %d test cases in plan %d — "+
				"ensure specs have CaseID metadata matching the plan's test cases",
			len(planCases), planID)
	}

	log.Infof("TCMS", "Matched %d local specs to plan cases (plan has %d cases total)",
		len(matchedCaseIDs), len(planCases))

	managerID, err := client.GetCurrentUserID()
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("getting current user: %w", err)
	}

	suiteName := "default"
	if suite.SuiteSpec != nil {
		suiteName = suite.SuiteSpec.Name
	}
	summary := fmt.Sprintf("SDT Run - %s - Plan %d - %s",
		suiteName, planID, time.Now().Format("2006-01-02 15:04"))
	runID, err := client.CreateTestRun(summary, planID, buildID, managerID)
	if err != nil {
		return 0, 0, nil, nil, fmt.Errorf("creating test run: %w", err)
	}

	for specCaseID, tcmsCaseID := range matchedCaseIDs {
		_, err := client.CreateTestExecution(runID, tcmsCaseID, buildID)
		if err != nil {
			log.Warnf("TCMS", "Failed to create execution for case %d: %v", tcmsCaseID, err)
			delete(matchedCaseIDs, specCaseID)
		}
	}

	log.Infof("TCMS", "Test run %d created with %d executions under plan %d",
		runID, len(matchedCaseIDs), planID)
	return runID, buildID, matchedCaseIDs, plan, nil
}

// CheckLinkage verifies that all specs have valid CaseIDs in Kiwi TCMS.
func CheckLinkage(client *KiwiClient, suite *spec.Suite) []LinkStatus {
	var statuses []LinkStatus
	for _, testSpec := range suite.Tests {
		status := LinkStatus{
			SpecName: testSpec.TestName(),
			FilePath: testSpec.FilePath,
			CaseID:   testSpec.Metadata.CaseID,
		}
		if testSpec.Metadata.CaseID == "" {
			status.Status = "missing"
			status.Message = "No CaseID in spec metadata"
		} else {
			caseID, err := strconv.Atoi(testSpec.Metadata.CaseID)
			if err != nil {
				status.Status = "invalid"
				status.Message = fmt.Sprintf("CaseID %q is not a number", testSpec.Metadata.CaseID)
			} else {
				cases, err := client.FilterTestCases(map[string]interface{}{"id": caseID})
				if err != nil {
					status.Status = "error"
					status.Message = err.Error()
				} else if len(cases) == 0 {
					status.Status = "unlinked"
					status.Message = fmt.Sprintf("Case %d not found in Kiwi TCMS", caseID)
				} else {
					status.Status = "linked"
					status.Message = cases[0].Summary
				}
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// ImportTestCases imports test cases from a Kiwi TCMS plan as markdown spec stubs.
func ImportTestCases(client *KiwiClient, planID int, outputDir string) error {
	// Get cases for the plan
	cases, err := client.FilterTestCases(map[string]interface{}{"plan": planID})
	if err != nil {
		return fmt.Errorf("fetching test cases for plan %d: %w", planID, err)
	}

	if len(cases) == 0 {
		return fmt.Errorf("no test cases found in plan %d", planID)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	for _, tc := range cases {
		filename := fmt.Sprintf("%s/%d.md", outputDir, tc.ID)
		content := fmt.Sprintf(`# Test: %s

## Metadata
- CaseID: %d
- Priority: %d

## Steps
%s

## Verify
- Verify the expected behavior.
`, tc.Summary, tc.ID, tc.Priority, tc.Text)

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			log.Warnf("TCMS", "Failed to write %s: %v", filename, err)
			continue
		}
		log.Infof("TCMS", "Imported case %d -> %s", tc.ID, filename)
	}

	log.Infof("TCMS", "Imported %d test cases to %s", len(cases), outputDir)
	return nil
}

// SyncResult holds the outcome of syncing specs to TCMS.
type SyncResult struct {
	PlanID int
	Synced []SyncedCase
	Errors []SyncError
}

// SyncedCase represents a successfully synced test case.
type SyncedCase struct {
	SpecName string
	CaseID   int
	FilePath string
}

// SyncError represents a failed sync attempt.
type SyncError struct {
	SpecName string
	Error    string
}

// LinkStatus represents the linkage status between a spec and Kiwi TCMS.
type LinkStatus struct {
	SpecName string
	FilePath string
	CaseID   string
	Status   string // "linked", "unlinked", "missing", "invalid", "error"
	Message  string
}

// updateSpecCaseID updates the CaseID line in a spec markdown file.
func updateSpecCaseID(filePath string, newCaseID int) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)
	re := regexp.MustCompile(`(?m)^([-*]\s*CaseID:\s*).*$`)
	newLine := fmt.Sprintf("${1}%d", newCaseID)

	updated := re.ReplaceAllString(content, newLine)
	if updated == content {
		return nil // no CaseID line found or already correct
	}

	return os.WriteFile(filePath, []byte(updated), 0644)
}
