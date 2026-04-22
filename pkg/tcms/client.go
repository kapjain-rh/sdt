package tcms

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/openshift/sdt/pkg/log"
	"github.com/openshift/sdt/pkg/reporter"
	"github.com/openshift/sdt/pkg/spec"
)

// KiwiClient communicates with Kiwi TCMS via its JSON-RPC API.
type KiwiClient struct {
	baseURL    string
	rpcURL     string
	username   string
	password   string
	httpClient *http.Client
	csrfToken  string
	requestID  atomic.Int64
}

// jsonRPCRequest is the JSON-RPC 2.0 request format.
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int64       `json:"id"`
}

// jsonRPCResponse is the JSON-RPC 2.0 response format.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	ID      int64           `json:"id"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewKiwiClient creates a Kiwi TCMS client from environment variables.
func NewKiwiClient() (*KiwiClient, error) {
	baseURL := os.Getenv("KIWI_TCMS_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("KIWI_TCMS_URL environment variable is required")
	}

	username := os.Getenv("KIWI_TCMS_USERNAME")
	password := os.Getenv("KIWI_TCMS_PASSWORD")
	if username == "" || password == "" {
		return nil, fmt.Errorf("KIWI_TCMS_USERNAME and KIWI_TCMS_PASSWORD are required")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	// Allow self-signed certificates (common for local Kiwi TCMS instances)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402
	}

	client := &KiwiClient{
		baseURL:  baseURL,
		rpcURL:   baseURL + "/json-rpc/",
		username: username,
		password: password,
		httpClient: &http.Client{
			Jar:       jar,
			Transport: transport,
		},
	}

	if err := client.login(); err != nil {
		return nil, fmt.Errorf("authenticating with Kiwi TCMS: %w", err)
	}

	log.Infof("TCMS", "Connected to Kiwi TCMS at %s as %s", baseURL, username)
	return client, nil
}

// login authenticates with Kiwi TCMS and stores the session cookie.
func (k *KiwiClient) login() error {
	// First, do a GET to obtain the CSRF cookie
	resp, err := k.httpClient.Get(k.baseURL + "/")
	if err != nil {
		return fmt.Errorf("fetching CSRF token: %w", err)
	}
	resp.Body.Close()

	u, _ := url.Parse(k.baseURL)
	for _, c := range k.httpClient.Jar.Cookies(u) {
		if c.Name == "csrftoken" {
			k.csrfToken = c.Value
			break
		}
	}

	result, err := k.call("Auth.login", []interface{}{k.username, k.password})
	if err != nil {
		return err
	}

	// Update CSRF token from response cookies
	for _, c := range k.httpClient.Jar.Cookies(u) {
		if c.Name == "csrftoken" {
			k.csrfToken = c.Value
			break
		}
	}

	var session string
	if err := json.Unmarshal(result, &session); err != nil {
		return fmt.Errorf("parsing login response: %w", err)
	}

	log.Debugf("TCMS", "Authenticated, session established")
	return nil
}

// call makes a JSON-RPC 2.0 call and returns the result.
func (k *KiwiClient) call(method string, params interface{}) (json.RawMessage, error) {
	id := k.requestID.Add(1)
	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, k.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if k.csrfToken != "" {
		req.Header.Set("X-CSRFToken", k.csrfToken)
		req.Header.Set("Referer", k.baseURL)
	}

	log.Debugf("TCMS", "RPC call: %s", method)

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// --- Product / Build / Version helpers ---

// GetOrCreateProduct finds a product by name, or returns an error if not found.
func (k *KiwiClient) GetOrCreateProduct(name string) (int, error) {
	result, err := k.call("Product.filter", []interface{}{map[string]string{"name": name}})
	if err != nil {
		return 0, err
	}

	var products []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(result, &products); err != nil {
		return 0, fmt.Errorf("parsing products: %w", err)
	}

	if len(products) > 0 {
		return products[0].ID, nil
	}

	return 0, fmt.Errorf("product %q not found in Kiwi TCMS — create it via the web UI first", name)
}

// GetVersionForProduct returns the first version for a product.
func (k *KiwiClient) GetVersionForProduct(productID int) (int, error) {
	result, err := k.call("Version.filter", []interface{}{map[string]int{"product": productID}})
	if err != nil {
		return 0, err
	}

	var versions []struct {
		ID    int    `json:"id"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(result, &versions); err != nil {
		return 0, fmt.Errorf("parsing versions: %w", err)
	}

	if len(versions) > 0 {
		return versions[0].ID, nil
	}

	return 0, fmt.Errorf("no version found for product %d", productID)
}

// GetOrCreateBuild finds or creates a build for a version.
func (k *KiwiClient) GetOrCreateBuild(versionID int, name string) (int, error) {
	result, err := k.call("Build.filter", []interface{}{map[string]interface{}{
		"version": versionID,
		"name":    name,
	}})
	if err != nil {
		return 0, err
	}

	var builds []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(result, &builds); err != nil {
		return 0, fmt.Errorf("parsing builds: %w", err)
	}

	if len(builds) > 0 {
		return builds[0].ID, nil
	}

	// Create new build
	result, err = k.call("Build.create", []interface{}{map[string]interface{}{
		"name":    name,
		"version": versionID,
	}})
	if err != nil {
		return 0, fmt.Errorf("creating build: %w", err)
	}

	var build struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &build); err != nil {
		return 0, fmt.Errorf("parsing new build: %w", err)
	}

	log.Infof("TCMS", "Created build %q (id=%d)", name, build.ID)
	return build.ID, nil
}

// GetCategory returns the first category for a product, or creates "SDT" if none exists.
func (k *KiwiClient) GetCategory(productID int) (int, error) {
	result, err := k.call("Category.filter", []interface{}{map[string]int{"product": productID}})
	if err != nil {
		return 0, err
	}

	var categories []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(result, &categories); err != nil {
		return 0, fmt.Errorf("parsing categories: %w", err)
	}

	if len(categories) > 0 {
		return categories[0].ID, nil
	}

	return 0, fmt.Errorf("no category found for product %d — create one via the Kiwi TCMS UI", productID)
}

// GetPriority returns the first priority, or 0 if none found.
func (k *KiwiClient) GetPriority(name string) (int, error) {
	result, err := k.call("Priority.filter", []interface{}{map[string]string{}})
	if err != nil {
		return 0, err
	}

	var priorities []struct {
		ID    int    `json:"id"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(result, &priorities); err != nil {
		return 0, fmt.Errorf("parsing priorities: %w", err)
	}

	// Try to match by name
	for _, p := range priorities {
		if p.Value == name {
			return p.ID, nil
		}
	}

	if len(priorities) > 0 {
		return priorities[0].ID, nil
	}

	return 0, fmt.Errorf("no priorities configured in Kiwi TCMS")
}

// getCaseStatusID returns the test case status ID for a given name (e.g., "CONFIRMED", "PROPOSED").
func (k *KiwiClient) getCaseStatusID(name string) (int, error) {
	result, err := k.call("TestCaseStatus.filter", []interface{}{map[string]string{"name": name}})
	if err != nil {
		return 0, err
	}

	var statuses []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(result, &statuses); err != nil {
		return 0, fmt.Errorf("parsing case statuses: %w", err)
	}

	if len(statuses) > 0 {
		return statuses[0].ID, nil
	}

	return 0, fmt.Errorf("case status %q not found", name)
}

// GetExecutionStatusID returns the status ID for a given status name (e.g., "PASSED", "FAILED").
func (k *KiwiClient) GetExecutionStatusID(name string) (int, error) {
	result, err := k.call("TestExecutionStatus.filter", []interface{}{map[string]string{"name": name}})
	if err != nil {
		return 0, err
	}

	var statuses []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(result, &statuses); err != nil {
		return 0, fmt.Errorf("parsing statuses: %w", err)
	}

	if len(statuses) > 0 {
		return statuses[0].ID, nil
	}

	return 0, fmt.Errorf("status %q not found", name)
}

// GetCurrentUserID returns the ID of the authenticated user.
func (k *KiwiClient) GetCurrentUserID() (int, error) {
	result, err := k.call("User.filter", []interface{}{map[string]string{"username": k.username}})
	if err != nil {
		return 0, err
	}

	var users []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &users); err != nil {
		return 0, fmt.Errorf("parsing users: %w", err)
	}

	if len(users) > 0 {
		return users[0].ID, nil
	}

	return 0, fmt.Errorf("user %q not found", k.username)
}

// --- Test Case operations ---

// SyncTestCase creates or updates a test case in Kiwi TCMS from a spec.
func (k *KiwiClient) SyncTestCase(s *spec.TestSpec, productID, categoryID, priorityID int) (int, error) {
	summary := s.TestName()

	// Build the test case text from spec steps
	text := ""
	if len(s.Setup) > 0 {
		text += "## Setup\n"
		for _, step := range s.Setup {
			text += "- " + step.RawText + "\n"
		}
		text += "\n"
	}
	if len(s.Steps) > 0 {
		text += "## Steps\n"
		for _, step := range s.Steps {
			text += "- " + step.RawText + "\n"
		}
		text += "\n"
	}
	if len(s.Verify) > 0 {
		text += "## Verify\n"
		for _, step := range s.Verify {
			text += "- " + step.RawText + "\n"
		}
	}

	// If spec has a CaseID, try to update
	if s.Metadata.CaseID != "" {
		caseID, err := strconv.Atoi(s.Metadata.CaseID)
		if err == nil {
			_, err := k.call("TestCase.update", []interface{}{caseID, map[string]interface{}{
				"summary":  summary,
				"text":     text,
				"priority": priorityID,
			}})
			if err == nil {
				log.Infof("TCMS", "Updated test case %d: %s", caseID, summary)
				return caseID, nil
			}
			log.Warnf("TCMS", "Failed to update case %d, will create new: %v", caseID, err)
		}
	}

	// Get CONFIRMED status for new test cases
	caseStatusID, err := k.getCaseStatusID("CONFIRMED")
	if err != nil {
		caseStatusID = 2 // fallback to common default
	}

	// Create new test case
	result, err := k.call("TestCase.create", []interface{}{map[string]interface{}{
		"summary":      summary,
		"product":      productID,
		"category":     categoryID,
		"priority":     priorityID,
		"case_status":  caseStatusID,
		"is_automated": true,
		"text":         text,
		"notes":        fmt.Sprintf("Synced from SDT spec: %s", s.FilePath),
	}})
	if err != nil {
		return 0, fmt.Errorf("creating test case: %w", err)
	}

	var tc struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &tc); err != nil {
		return 0, fmt.Errorf("parsing test case: %w", err)
	}

	log.Infof("TCMS", "Created test case %d: %s", tc.ID, summary)
	return tc.ID, nil
}

// FilterTestCases returns test cases matching a query.
func (k *KiwiClient) FilterTestCases(query map[string]interface{}) ([]TestCaseInfo, error) {
	result, err := k.call("TestCase.filter", []interface{}{query})
	if err != nil {
		return nil, err
	}

	var cases []TestCaseInfo
	if err := json.Unmarshal(result, &cases); err != nil {
		return nil, fmt.Errorf("parsing test cases: %w", err)
	}

	return cases, nil
}

// TestCaseInfo holds test case information from Kiwi TCMS.
type TestCaseInfo struct {
	ID          int    `json:"id"`
	Summary     string `json:"summary"`
	Text        string `json:"text"`
	Priority    int    `json:"priority"`
	IsAutomated bool   `json:"is_automated"`
	Script      string `json:"script"`
	Notes       string `json:"notes"`
}

// TestPlanInfo holds test plan information from Kiwi TCMS.
type TestPlanInfo struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Product        int    `json:"product"`
	ProductVersion int    `json:"product_version"`
}

// --- Test Plan operations ---

// GetTestPlan fetches a test plan by ID from Kiwi TCMS.
func (k *KiwiClient) GetTestPlan(planID int) (*TestPlanInfo, error) {
	result, err := k.call("TestPlan.filter", []interface{}{map[string]interface{}{"id": planID}})
	if err != nil {
		return nil, err
	}

	var plans []TestPlanInfo
	if err := json.Unmarshal(result, &plans); err != nil {
		return nil, fmt.Errorf("parsing test plan: %w", err)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("test plan %d not found in Kiwi TCMS", planID)
	}

	return &plans[0], nil
}

// GetTestCasesForPlan returns all test cases linked to a test plan.
func (k *KiwiClient) GetTestCasesForPlan(planID int) ([]TestCaseInfo, error) {
	return k.FilterTestCases(map[string]interface{}{"plan": planID})
}

// GetOrCreateTestPlan finds or creates a test plan.
func (k *KiwiClient) GetOrCreateTestPlan(name string, productID, versionID, planTypeID int) (int, error) {
	result, err := k.call("TestPlan.filter", []interface{}{map[string]interface{}{
		"name":    name,
		"product": productID,
	}})
	if err != nil {
		return 0, err
	}

	var plans []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &plans); err != nil {
		return 0, fmt.Errorf("parsing plans: %w", err)
	}

	if len(plans) > 0 {
		return plans[0].ID, nil
	}

	// Create new plan
	result, err = k.call("TestPlan.create", []interface{}{map[string]interface{}{
		"name":            name,
		"product":         productID,
		"product_version": versionID,
		"type":            planTypeID,
	}})
	if err != nil {
		return 0, fmt.Errorf("creating test plan: %w", err)
	}

	var plan struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &plan); err != nil {
		return 0, fmt.Errorf("parsing new plan: %w", err)
	}

	log.Infof("TCMS", "Created test plan %q (id=%d)", name, plan.ID)
	return plan.ID, nil
}

// AddCaseToPlan adds a test case to a test plan.
func (k *KiwiClient) AddCaseToPlan(planID, caseID int) error {
	_, err := k.call("TestPlan.add_case", []interface{}{planID, caseID})
	return err
}

// GetPlanType returns the first plan type ID.
func (k *KiwiClient) GetPlanType() (int, error) {
	result, err := k.call("PlanType.filter", []interface{}{map[string]string{}})
	if err != nil {
		return 0, err
	}

	var types []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &types); err != nil {
		return 0, fmt.Errorf("parsing plan types: %w", err)
	}

	if len(types) > 0 {
		return types[0].ID, nil
	}

	return 0, fmt.Errorf("no plan types configured in Kiwi TCMS")
}

// --- Test Run operations ---

// CreateTestRun creates a new test run in Kiwi TCMS.
func (k *KiwiClient) CreateTestRun(summary string, planID, buildID, managerID int) (int, error) {
	result, err := k.call("TestRun.create", []interface{}{map[string]interface{}{
		"summary": summary,
		"plan":    planID,
		"build":   buildID,
		"manager": managerID,
	}})
	if err != nil {
		return 0, fmt.Errorf("creating test run: %w", err)
	}

	var run struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &run); err != nil {
		return 0, fmt.Errorf("parsing test run: %w", err)
	}

	log.Infof("TCMS", "Created test run %q (id=%d)", summary, run.ID)
	return run.ID, nil
}

// FilterTestRuns returns test runs matching a query.
func (k *KiwiClient) FilterTestRuns(query map[string]interface{}) ([]TestRunInfo, error) {
	result, err := k.call("TestRun.filter", []interface{}{query})
	if err != nil {
		return nil, err
	}

	var runs []TestRunInfo
	if err := json.Unmarshal(result, &runs); err != nil {
		return nil, fmt.Errorf("parsing test runs: %w", err)
	}

	return runs, nil
}

// TestRunInfo holds test run information from Kiwi TCMS.
type TestRunInfo struct {
	ID      int    `json:"id"`
	Summary string `json:"summary"`
	PlanID  int    `json:"plan"`
	BuildID int    `json:"build"`
}

// --- Test Execution operations ---

// CreateTestExecution adds a test case to a test run as a test execution.
func (k *KiwiClient) CreateTestExecution(runID, caseID, buildID int) (int, error) {
	// Get IDLE status
	statusID, err := k.GetExecutionStatusID("IDLE")
	if err != nil {
		return 0, err
	}

	result, err := k.call("TestExecution.create", []interface{}{map[string]interface{}{
		"run":    runID,
		"case":   caseID,
		"status": statusID,
		"build":  buildID,
	}})
	if err != nil {
		return 0, fmt.Errorf("creating test execution: %w", err)
	}

	var exec struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(result, &exec); err != nil {
		return 0, fmt.Errorf("parsing test execution: %w", err)
	}

	return exec.ID, nil
}

// UpdateTestExecution updates a test execution's status.
func (k *KiwiClient) UpdateTestExecution(executionID, statusID int) error {
	_, err := k.call("TestExecution.update", []interface{}{executionID, map[string]interface{}{
		"status": statusID,
	}})
	return err
}

// FilterTestExecutions returns test executions matching a query.
func (k *KiwiClient) FilterTestExecutions(query map[string]interface{}) ([]TestExecutionInfo, error) {
	result, err := k.call("TestExecution.filter", []interface{}{query})
	if err != nil {
		return nil, err
	}

	var execs []TestExecutionInfo
	if err := json.Unmarshal(result, &execs); err != nil {
		return nil, fmt.Errorf("parsing test executions: %w", err)
	}

	return execs, nil
}

// TestExecutionInfo holds test execution information from Kiwi TCMS.
type TestExecutionInfo struct {
	ID         int    `json:"id"`
	RunID      int    `json:"run"`
	CaseID     int    `json:"case"`
	StatusName string `json:"status__name"`
	StatusID   int    `json:"status"`
}

// --- Reporting ---

// ReportResult reports a test execution result to a Kiwi test run.
func (k *KiwiClient) ReportResult(executionID int, report *reporter.TestReport) error {
	statusName := "IDLE"
	switch report.Status {
	case reporter.TestPassed:
		statusName = "PASSED"
	case reporter.TestFailed:
		statusName = "FAILED"
	case reporter.TestSkipped:
		statusName = "WAIVED"
	}

	statusID, err := k.GetExecutionStatusID(statusName)
	if err != nil {
		return fmt.Errorf("getting status ID for %s: %w", statusName, err)
	}

	if err := k.UpdateTestExecution(executionID, statusID); err != nil {
		return fmt.Errorf("updating execution %d: %w", executionID, err)
	}

	log.Infof("TCMS", "Reported execution %d as %s", executionID, statusName)
	return nil
}
