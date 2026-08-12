package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sdt-project/sdt/pkg/mcp"
)

type Server struct {
	store        *ProjectStore
	toolExecutor *ToolExecutor
	specExecutor *SpecExecutor
	mcpManager   *MCPManager
	mux          *http.ServeMux
	uiFS         http.Handler
	uiDist       fs.FS
	projectDir   string
}

func NewServer(projectDir string) (*Server, error) {
	store, err := NewProjectStore(projectDir)
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	s := &Server{
		store:      store,
		mcpManager: NewMCPManager(),
		mux:        http.NewServeMux(),
		projectDir: projectDir,
	}
	s.toolExecutor = NewToolExecutor(store)
	s.specExecutor = NewSpecExecutor(store, projectDir)
	s.registerRoutes()
	return s, nil
}

func (s *Server) Shutdown() {
	s.mcpManager.CloseAll()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}

	// API routes
	if strings.HasPrefix(r.URL.Path, "/api/") {
		log.Printf("%s %s", r.Method, r.URL.Path)
		s.mux.ServeHTTP(w, r)
		return
	}

	// UI routes (when --ui is enabled)
	if s.uiFS != nil {
		s.serveUI(w, r)
		return
	}

	// No UI — only API is available
	log.Printf("%s %s", r.Method, r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/api/dashboard/trends", s.handleTrends)
	s.mux.HandleFunc("/api/projects", s.handleProjects)
	s.mux.HandleFunc("/api/projects/", s.handleProject)
	s.mux.HandleFunc("/api/cases/", s.handleCase)
	s.mux.HandleFunc("/api/plans/", s.handlePlan)
	s.mux.HandleFunc("/api/runs/", s.handleRun)
	s.mux.HandleFunc("/api/results/", s.handleResult)
	s.mux.HandleFunc("/api/executions/", s.handleExecution)
	s.mux.HandleFunc("/api/tools/", s.handleTool)
	s.mux.HandleFunc("/api/tool-runs/", s.handleToolRun)
	s.mux.HandleFunc("/api/mcp-servers/", s.handleMCPServer)
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func pathSegments(path, prefix string) []string {
	s := strings.TrimPrefix(path, prefix)
	s = strings.Trim(s, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

// --- Dashboard ---

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetDashboardStats()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, stats)
}

func (s *Server) handleTrends(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days == 0 {
		days = 30
	}
	trends, err := s.store.GetTrends(days)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if trends == nil {
		trends = []TrendPoint{}
	}
	writeJSON(w, 200, trends)
}

// --- Projects ---

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects := s.store.ListProjects()
		writeJSON(w, 200, projects)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/projects/")
	if len(segs) == 0 {
		writeError(w, 400, "missing project id")
		return
	}
	_, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid project id")
		return
	}

	if len(segs) >= 2 && segs[1] == "cases" {
		s.handleProjectCases(w, r)
		return
	}
	if len(segs) >= 2 && segs[1] == "plans" {
		s.handleProjectPlans(w, r)
		return
	}
	if len(segs) >= 2 && segs[1] == "runs" {
		s.handleProjectRuns(w, r)
		return
	}
	if len(segs) >= 2 && segs[1] == "mcp-servers" {
		s.handleProjectMCPServers(w, r)
		return
	}
	if len(segs) >= 3 && segs[1] == "suite" && segs[2] == "cache" {
		s.handleSuiteCache(w, r)
		return
	}
	if len(segs) >= 2 && segs[1] == "suite" {
		s.handleProjectSuite(w, r)
		return
	}
	if len(segs) >= 2 && segs[1] == "groups" {
		s.handleProjectGroups(w, r, segs[2:])
		return
	}
	if len(segs) >= 2 && segs[1] == "fixtures" {
		s.handleProjectFixtures(w, r, segs[2:])
		return
	}
	if len(segs) >= 2 && segs[1] == "cache" {
		s.handleProjectCache(w, r)
		return
	}
	if len(segs) >= 3 && segs[1] == "tools" && segs[2] == "export" {
		s.handleToolsExport(w, r)
		return
	}
	if len(segs) >= 3 && segs[1] == "tools" && segs[2] == "llm" {
		s.handleToolsLLM(w, r)
		return
	}
	if len(segs) >= 2 && segs[1] == "tools" {
		s.handleProjectTools(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		p := s.store.GetProject()
		writeJSON(w, 200, p)
	case http.MethodPut:
		var p Project
		if err := readJSON(r, &p); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if err := s.store.UpdateProject(&p); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, s.store.GetProject())
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleProjectCases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		search := r.URL.Query().Get("search")
		cases, err := s.store.ListCases(search)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if cases == nil {
			cases = []TestCase{}
		}
		writeJSON(w, 200, cases)
	case http.MethodPost:
		var c TestCase
		if err := readJSON(r, &c); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		c.ProjectID = 1
		if c.Title == "" {
			writeError(w, 400, "title is required")
			return
		}
		if c.Status == "" {
			c.Status = "draft"
		}
		if c.Priority == "" {
			c.Priority = "Medium"
		}
		if err := s.store.CreateCase(&c); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, c)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleProjectPlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		plans, err := s.store.ListPlans()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if plans == nil {
			plans = []TestPlan{}
		}
		writeJSON(w, 200, plans)
	case http.MethodPost:
		var p TestPlan
		if err := readJSON(r, &p); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		p.ProjectID = 1
		if p.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		if p.Status == "" {
			p.Status = "active"
		}
		if err := s.store.CreatePlan(&p); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, p)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleProjectRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListAllRuns()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if runs == nil {
		runs = []TestRun{}
	}
	writeJSON(w, 200, runs)
}

// --- Cases ---

func (s *Server) handleCase(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/cases/")
	if len(segs) == 0 {
		writeError(w, 400, "missing case id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid case id")
		return
	}

	if len(segs) >= 2 && segs[1] == "execute" && r.Method == http.MethodPost {
		var body struct {
			ExecutedBy string            `json:"executed_by"`
			Title      string            `json:"title"`
			Steps      []string          `json:"steps"`
			EnvVars    map[string]string `json:"env_vars"`
		}
		readJSON(r, &body)

		execution := &Execution{
			CaseID:     id,
			Status:     "pending",
			ExecutedBy: body.ExecutedBy,
			EnvVars:    body.EnvVars,
		}
		if err := s.store.CreateExecution(execution); err != nil {
			writeError(w, 500, err.Error())
			return
		}

		if len(body.Steps) > 0 {
			s.specExecutor.RunSteps(execution, body.Title, body.Steps)
		} else {
			s.specExecutor.RunCase(execution)
		}

		writeJSON(w, 201, map[string]interface{}{"execution_id": execution.ID})
		return
	}

	if len(segs) >= 2 && segs[1] == "cache" {
		switch r.Method {
		case http.MethodGet:
			cc, err := s.store.GetCacheForCase(id)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, cc)
			return
		case http.MethodPost:
			if err := s.specExecutor.SaveCacheForCase(id); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, map[string]string{"status": "cached"})
			return
		case http.MethodDelete:
			if err := s.store.DeleteCacheForCase(id); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, map[string]string{"status": "deleted"})
			return
		}
	}

	if len(segs) >= 2 && segs[1] == "executions" && r.Method == http.MethodGet {
		execs, err := s.store.ListExecutions(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if execs == nil {
			execs = []Execution{}
		}
		writeJSON(w, 200, execs)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c, err := s.store.GetCase(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if c == nil {
			writeError(w, 404, "case not found")
			return
		}
		writeJSON(w, 200, c)
	case http.MethodPut:
		var c TestCase
		if err := readJSON(r, &c); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		c.ID = id
		if err := s.store.UpdateCase(&c); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, c)
	case http.MethodDelete:
		if err := s.store.DeleteCase(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeError(w, 405, "method not allowed")
	}
}

// --- Plans ---

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/plans/")
	if len(segs) == 0 {
		writeError(w, 400, "missing plan id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid plan id")
		return
	}

	if len(segs) >= 2 && segs[1] == "cases" {
		s.handlePlanCases(w, r, id, segs)
		return
	}
	if len(segs) >= 2 && segs[1] == "runs" {
		s.handlePlanRuns(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, err := s.store.GetPlan(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if p == nil {
			writeError(w, 404, "plan not found")
			return
		}
		writeJSON(w, 200, p)
	case http.MethodPut:
		var p TestPlan
		if err := readJSON(r, &p); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		p.ID = id
		if err := s.store.UpdatePlan(&p); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, p)
	case http.MethodDelete:
		if err := s.store.DeletePlan(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handlePlanCases(w http.ResponseWriter, r *http.Request, planID int64, segs []string) {
	switch r.Method {
	case http.MethodGet:
		cases, err := s.store.GetPlanCases(planID)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if cases == nil {
			cases = []TestCase{}
		}
		writeJSON(w, 200, cases)
	case http.MethodPost:
		var body struct {
			CaseIDs []int64 `json:"case_ids"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if err := s.store.SetPlanCases(planID, body.CaseIDs); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "updated"})
	case http.MethodDelete:
		if len(segs) >= 3 {
			caseID, _ := strconv.ParseInt(segs[2], 10, 64)
			if err := s.store.RemoveCaseFromPlan(planID, caseID); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, map[string]string{"status": "removed"})
		}
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handlePlanRuns(w http.ResponseWriter, r *http.Request, planID int64) {
	switch r.Method {
	case http.MethodGet:
		runs, err := s.store.ListRuns(planID)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if runs == nil {
			runs = []TestRun{}
		}
		writeJSON(w, 200, runs)
	case http.MethodPost:
		var run TestRun
		if err := readJSON(r, &run); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		run.PlanID = planID
		if run.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		if err := s.store.CreateRun(&run); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, run)
	default:
		writeError(w, 405, "method not allowed")
	}
}

// --- Runs ---

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/runs/")
	if len(segs) == 0 {
		writeError(w, 400, "missing run id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid run id")
		return
	}

	if len(segs) >= 2 && segs[1] == "start" && r.Method == http.MethodPost {
		if err := s.store.StartRun(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "started"})
		return
	}
	if len(segs) >= 2 && segs[1] == "complete" && r.Method == http.MethodPost {
		if err := s.store.CompleteRun(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "completed"})
		return
	}
	if len(segs) >= 2 && segs[1] == "execute" && r.Method == http.MethodPost {
		var body struct {
			ExecutedBy string            `json:"executed_by"`
			EnvVars    map[string]string `json:"env_vars"`
		}
		readJSON(r, &body)
		s.specExecutor.RunBatch(id, body.ExecutedBy, body.EnvVars)
		writeJSON(w, 200, map[string]string{"status": "executing"})
		return
	}
	if len(segs) >= 2 && segs[1] == "stream" && r.Method == http.MethodGet {
		s.handleRunStream(w, r, id)
		return
	}
	if len(segs) >= 2 && segs[1] == "active-execution" && r.Method == http.MethodGet {
		execID := s.specExecutor.ActiveExecution(id)
		if execID == 0 {
			writeJSON(w, 200, map[string]interface{}{"execution_id": nil})
		} else {
			writeJSON(w, 200, map[string]interface{}{"execution_id": execID})
		}
		return
	}
	if len(segs) >= 2 && segs[1] == "results" {
		results, err := s.store.ListResults(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if results == nil {
			results = []TestResult{}
		}
		writeJSON(w, 200, results)
		return
	}

	switch r.Method {
	case http.MethodGet:
		run, err := s.store.GetRun(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if run == nil {
			writeError(w, 404, "run not found")
			return
		}
		writeJSON(w, 200, run)
	case http.MethodDelete:
		if err := s.store.DeleteRun(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleRunStream(w http.ResponseWriter, r *http.Request, runID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(200)
	flusher.Flush()

	eventIndex := 0
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		events := s.specExecutor.GetRunEvents(runID, eventIndex)
		for _, ev := range events {
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			eventIndex++
		}
		if len(events) > 0 {
			flusher.Flush()
		}

		isDone := false
		for _, ev := range events {
			if ev.Type == "done" {
				isDone = true
				break
			}
		}
		if isDone {
			return
		}

		if !s.specExecutor.IsRunActive(runID) && len(events) == 0 {
			data, _ := json.Marshal(map[string]string{"type": "done"})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// --- Results ---

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/results/")
	if len(segs) == 0 {
		writeError(w, 400, "missing result id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid result id")
		return
	}

	if len(segs) >= 2 && segs[1] == "steps" && r.Method == http.MethodPost {
		var steps []TestStepResult
		if err := readJSON(r, &steps); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if err := s.store.SaveStepResults(id, steps); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "saved"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		result, err := s.store.GetResult(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if result == nil {
			writeError(w, 404, "result not found")
			return
		}
		writeJSON(w, 200, result)
	case http.MethodPut:
		var body struct {
			Status     string           `json:"status"`
			Comment    string           `json:"comment"`
			ExecutedBy string           `json:"executed_by"`
			DurationMs int64            `json:"duration_ms"`
			Steps      []TestStepResult `json:"step_results"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		result, err := s.store.GetResult(id)
		if err != nil || result == nil {
			writeError(w, 404, "result not found")
			return
		}
		result.Status = body.Status
		result.Comment = body.Comment
		result.ExecutedBy = body.ExecutedBy
		result.DurationMs = body.DurationMs
		if err := s.store.UpdateResult(result); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if len(body.Steps) > 0 {
			if err := s.store.SaveStepResults(id, body.Steps); err != nil {
				writeError(w, 500, fmt.Sprintf("saving step results: %v", err))
				return
			}
		}
		updated, _ := s.store.GetResult(id)
		writeJSON(w, 200, updated)
	default:
		writeError(w, 405, "method not allowed")
	}
}

// --- Executions ---

func (s *Server) handleExecution(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/executions/")
	if len(segs) == 0 {
		writeError(w, 400, "missing execution id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid execution id")
		return
	}

	if len(segs) >= 2 && segs[1] == "stream" && r.Method == http.MethodGet {
		s.handleExecutionStream(w, r, id)
		return
	}

	if len(segs) >= 2 && segs[1] == "logs" && r.Method == http.MethodGet {
		afterStr := r.URL.Query().Get("after")
		afterID, _ := strconv.ParseInt(afterStr, 10, 64)
		logs, err := s.store.GetLogsSince(id, afterID)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if logs == nil {
			logs = []ExecutionLog{}
		}
		writeJSON(w, 200, logs)
		return
	}

	if r.Method == http.MethodGet {
		exec, err := s.store.GetExecution(id)
		if err != nil {
			writeError(w, 404, "execution not found")
			return
		}
		writeJSON(w, 200, exec)
		return
	}

	writeError(w, 405, "method not allowed")
}

func (s *Server) handleExecutionStream(w http.ResponseWriter, r *http.Request, execID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(200)
	flusher.Flush()

	var lastLogID int64
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		logs, err := s.store.GetLogsSince(execID, lastLogID)
		if err == nil && len(logs) > 0 {
			for _, l := range logs {
				data, _ := json.Marshal(l)
				fmt.Fprintf(w, "data: %s\n\n", data)
				lastLogID = l.ID
			}
			flusher.Flush()
		}

		exec, err := s.store.GetExecution(execID)
		if err == nil && (exec.Status != "pending" && exec.Status != "running") {
			data, _ := json.Marshal(map[string]interface{}{
				"type":     "done",
				"status":   exec.Status,
				"verdict":  exec.Verdict,
				"duration": exec.DurationMs,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// --- Suite, Groups, Fixtures ---

func (s *Server) handleProjectSuite(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		suite, err := s.store.GetSuite()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if suite == nil {
			writeJSON(w, 200, map[string]interface{}{})
			return
		}
		writeJSON(w, 200, suite)
	case http.MethodPut:
		var info SuiteInfo
		if err := readJSON(r, &info); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if err := s.store.SaveSuite(&info); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, info)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleSuiteCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}
	cc, err := s.store.GetCacheForSuite()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, cc)
}

func (s *Server) handleProjectGroups(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) > 0 {
		groupName := rest[0]

		if len(rest) >= 2 && rest[1] == "cache" && r.Method == http.MethodGet {
			cc, err := s.store.GetCacheForGroup(groupName)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, cc)
			return
		}

		switch r.Method {
		case http.MethodGet:
			group, err := s.store.GetGroup(groupName)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			if group == nil {
				writeError(w, 404, "group not found")
				return
			}
			writeJSON(w, 200, group)
		case http.MethodPut:
			var info GroupInfo
			if err := readJSON(r, &info); err != nil {
				writeError(w, 400, "invalid JSON")
				return
			}
			if err := s.store.UpdateGroup(groupName, &info); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, info)
		case http.MethodDelete:
			if err := s.store.DeleteGroup(groupName); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, map[string]string{"status": "deleted"})
		default:
			writeError(w, 405, "method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		groups, err := s.store.ListGroups()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if groups == nil {
			groups = []GroupInfo{}
		}
		writeJSON(w, 200, groups)
	case http.MethodPost:
		var info GroupInfo
		if err := readJSON(r, &info); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if info.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		if err := s.store.CreateGroup(&info); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, info)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleProjectCache(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		summary, err := s.store.GetCacheSummary()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, summary)
	case http.MethodDelete:
		if err := s.store.ClearCache(); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "cleared"})
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleProjectFixtures(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) > 0 {
		fixtureName := rest[0]
		switch r.Method {
		case http.MethodGet:
			f, err := s.store.GetFixture(fixtureName)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			if f == nil {
				writeError(w, 404, "fixture not found")
				return
			}
			writeJSON(w, 200, f)
		case http.MethodPut:
			var f Fixture
			if err := readJSON(r, &f); err != nil {
				writeError(w, 400, "invalid JSON")
				return
			}
			if err := s.store.UpdateFixture(fixtureName, &f); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, f)
		case http.MethodDelete:
			if err := s.store.DeleteFixture(fixtureName); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, map[string]string{"status": "deleted"})
		default:
			writeError(w, 405, "method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		fixtures, err := s.store.ListFixtures()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if fixtures == nil {
			fixtures = []Fixture{}
		}
		writeJSON(w, 200, fixtures)
	case http.MethodPost:
		var f Fixture
		if err := readJSON(r, &f); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if f.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		if f.Status == "" {
			f.Status = "draft"
		}
		if err := s.store.CreateFixture(&f); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, f)
	default:
		writeError(w, 405, "method not allowed")
	}
}

// --- Tools ---

func (s *Server) handleProjectTools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		search := r.URL.Query().Get("search")
		tools, err := s.store.ListTools(search)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if tools == nil {
			tools = []Tool{}
		}
		writeJSON(w, 200, tools)
	case http.MethodPost:
		var t Tool
		if err := readJSON(r, &t); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		t.ProjectID = 1
		if t.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		if t.Status == "" {
			t.Status = "draft"
		}
		if err := s.store.CreateTool(&t); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, t)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleToolsExport(w http.ResponseWriter, r *http.Request) {
	tools, err := s.store.ListApprovedTools()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	type yamlParam struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
		Default     string `json:"default,omitempty"`
	}
	type yamlTool struct {
		Name        string               `json:"name"`
		Description string               `json:"description"`
		Category    string               `json:"category"`
		Status      string               `json:"status"`
		Input       map[string]yamlParam `json:"input,omitempty"`
		Command     string               `json:"command"`
	}

	var out []yamlTool
	for _, t := range tools {
		yt := yamlTool{
			Name:        t.Name,
			Description: t.Description,
			Category:    t.Category,
			Status:      t.Status,
		}
		cmd := t.Command
		for _, a := range t.Args {
			cmd += " " + a
		}
		yt.Command = cmd

		if len(t.InputParams) > 0 {
			yt.Input = make(map[string]yamlParam)
			for k, v := range t.InputParams {
				yt.Input[k] = yamlParam{
					Type:        v.Type,
					Description: v.Description,
					Required:    v.Required,
					Default:     v.Default,
				}
			}
		}
		out = append(out, yt)
	}
	if out == nil {
		out = []yamlTool{}
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleToolsLLM(w http.ResponseWriter, r *http.Request) {
	tools, err := s.store.ListApprovedTools()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	type llmTool struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		InputSchema interface{} `json:"input_schema"`
	}

	var out []llmTool
	for _, t := range tools {
		properties := map[string]interface{}{}
		var required []string
		for k, v := range t.InputParams {
			properties[k] = map[string]string{
				"type":        v.Type,
				"description": v.Description,
			}
			if v.Required {
				required = append(required, k)
			}
		}

		schema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			schema["required"] = required
		}

		desc := t.Description
		cmd := t.Command
		for _, a := range t.Args {
			cmd += " " + a
		}
		desc += fmt.Sprintf(" [%s: %s]", t.Category, cmd)

		out = append(out, llmTool{
			Name:        t.Name,
			Description: desc,
			InputSchema: schema,
		})
	}

	mcpServers, _ := s.store.ListMCPServers()
	var connectedIDs []int64
	serverNames := make(map[int64]string)
	for _, srv := range mcpServers {
		if s.mcpManager.IsConnected(srv.ID) {
			connectedIDs = append(connectedIDs, srv.ID)
			serverNames[srv.ID] = srv.Name
		}
	}
	allMCPTools := s.mcpManager.GetAllToolsForServers(connectedIDs)
	for serverID, mcpTools := range allMCPTools {
		sname := serverNames[serverID]
		for _, mt := range mcpTools {
			out = append(out, llmTool{
				Name:        sname + "__" + mt.Name,
				Description: mt.Description,
				InputSchema: mt.InputSchema,
			})
		}
	}

	if out == nil {
		out = []llmTool{}
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleTool(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/tools/")
	if len(segs) == 0 {
		writeError(w, 400, "missing tool id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid tool id")
		return
	}

	if len(segs) >= 2 && segs[1] == "test" && r.Method == http.MethodPost {
		t, err := s.store.GetTool(id)
		if err != nil || t == nil {
			writeError(w, 404, "tool not found")
			return
		}
		var body struct {
			Params map[string]string `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		runID, err := s.toolExecutor.Run(t, body.Params)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, map[string]interface{}{"run_id": runID})
		return
	}

	if len(segs) >= 2 && segs[1] == "runs" && r.Method == http.MethodGet {
		runs, err := s.store.ListToolRuns(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if runs == nil {
			runs = []ToolRun{}
		}
		writeJSON(w, 200, runs)
		return
	}

	if len(segs) >= 2 && segs[1] == "status" && r.Method == http.MethodPost {
		var body struct {
			Status string `json:"status"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		valid := map[string]bool{"draft": true, "verify": true, "approved": true}
		if !valid[body.Status] {
			writeError(w, 400, "status must be draft, verify, or approved")
			return
		}
		if err := s.store.UpdateToolStatus(id, body.Status); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		t, _ := s.store.GetTool(id)
		writeJSON(w, 200, t)
		return
	}

	switch r.Method {
	case http.MethodGet:
		t, err := s.store.GetTool(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if t == nil {
			writeError(w, 404, "tool not found")
			return
		}
		writeJSON(w, 200, t)
	case http.MethodPut:
		var t Tool
		if err := readJSON(r, &t); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		t.ID = id
		if err := s.store.UpdateTool(&t); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		updated, _ := s.store.GetTool(id)
		writeJSON(w, 200, updated)
	case http.MethodDelete:
		if err := s.store.DeleteTool(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeError(w, 405, "method not allowed")
	}
}

// --- Tool Runs ---

func (s *Server) handleToolRun(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/tool-runs/")
	if len(segs) == 0 {
		writeError(w, 400, "missing run id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid run id")
		return
	}

	if len(segs) >= 2 && segs[1] == "stream" && r.Method == http.MethodGet {
		s.handleToolRunStream(w, r, id)
		return
	}

	if len(segs) >= 2 && segs[1] == "logs" && r.Method == http.MethodGet {
		afterStr := r.URL.Query().Get("after")
		afterID, _ := strconv.ParseInt(afterStr, 10, 64)
		logs, err := s.store.GetToolLogsSince(id, afterID)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if logs == nil {
			logs = []ToolRunLog{}
		}
		writeJSON(w, 200, logs)
		return
	}

	if r.Method == http.MethodGet {
		run, err := s.store.GetToolRun(id)
		if err != nil || run == nil {
			writeError(w, 404, "tool run not found")
			return
		}
		writeJSON(w, 200, run)
		return
	}

	writeError(w, 405, "method not allowed")
}

func (s *Server) handleToolRunStream(w http.ResponseWriter, r *http.Request, runID int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(200)
	flusher.Flush()

	var lastLogID int64
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		logs, err := s.store.GetToolLogsSince(runID, lastLogID)
		if err == nil && len(logs) > 0 {
			for _, l := range logs {
				data, _ := json.Marshal(l)
				fmt.Fprintf(w, "data: %s\n\n", data)
				lastLogID = l.ID
			}
			flusher.Flush()
		}

		run, err := s.store.GetToolRun(runID)
		if err == nil && run != nil && (run.Status != "pending" && run.Status != "running") {
			data, _ := json.Marshal(map[string]interface{}{
				"type":      "done",
				"status":    run.Status,
				"exit_code": run.ExitCode,
				"duration":  run.DurationMs,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		time.Sleep(200 * time.Millisecond)
	}
}

// --- MCP Servers ---

func (s *Server) handleProjectMCPServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers, err := s.store.ListMCPServers()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if servers == nil {
			servers = []MCPServer{}
		}
		for i := range servers {
			if s.mcpManager.IsConnected(servers[i].ID) {
				servers[i].Status = "connected"
			}
		}
		writeJSON(w, 200, servers)
	case http.MethodPost:
		var srv MCPServer
		if err := readJSON(r, &srv); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		srv.ProjectID = 1
		if srv.Name == "" {
			writeError(w, 400, "name is required")
			return
		}
		if srv.Command == "" {
			writeError(w, 400, "command is required")
			return
		}
		if err := s.store.CreateMCPServer(&srv); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		srv.Status = "configured"
		writeJSON(w, 201, srv)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleMCPServer(w http.ResponseWriter, r *http.Request) {
	segs := pathSegments(r.URL.Path, "/api/mcp-servers/")
	if len(segs) == 0 {
		writeError(w, 400, "missing server id")
		return
	}
	id, err := strconv.ParseInt(segs[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid server id")
		return
	}

	if len(segs) >= 2 && segs[1] == "connect" && r.Method == http.MethodPost {
		srv, err := s.store.GetMCPServer(id)
		if err != nil || srv == nil {
			writeError(w, 404, "server not found")
			return
		}
		cfg := mcp.MCPServerConfig{
			Command: srv.Command,
			Args:    srv.Args,
			Env:     srv.Env,
			Dir:     s.projectDir,
		}
		tools, err := s.mcpManager.Connect(id, srv.Name, cfg)
		if err != nil {
			writeJSON(w, 200, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"tools":  []interface{}{},
			})
			return
		}
		writeJSON(w, 200, map[string]interface{}{
			"status": "connected",
			"tools":  tools,
		})
		return
	}

	if len(segs) >= 2 && segs[1] == "disconnect" && r.Method == http.MethodPost {
		s.mcpManager.Disconnect(id)
		writeJSON(w, 200, map[string]string{"status": "disconnected"})
		return
	}

	if len(segs) >= 2 && segs[1] == "tools" && r.Method == http.MethodGet {
		tools, ok := s.mcpManager.GetTools(id)
		if !ok {
			writeError(w, 400, "server not connected")
			return
		}
		writeJSON(w, 200, tools)
		return
	}

	if len(segs) >= 3 && segs[1] == "tools" && segs[2] == "call" && r.Method == http.MethodPost {
		srv, err := s.store.GetMCPServer(id)
		if err != nil || srv == nil {
			writeError(w, 404, "server not found")
			return
		}
		var body struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		if body.Arguments == nil {
			body.Arguments = json.RawMessage("{}")
		}
		runID, err := s.toolExecutor.RunMCPTool(s.mcpManager, id, srv.Name, body.Name, body.Arguments)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, map[string]interface{}{"run_id": runID})
		return
	}

	if len(segs) >= 2 && segs[1] == "refresh" && r.Method == http.MethodPost {
		tools, err := s.mcpManager.RefreshTools(id)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, tools)
		return
	}

	switch r.Method {
	case http.MethodGet:
		srv, err := s.store.GetMCPServer(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if srv == nil {
			writeError(w, 404, "server not found")
			return
		}
		if s.mcpManager.IsConnected(id) {
			srv.Status = "connected"
		}
		writeJSON(w, 200, srv)
	case http.MethodPut:
		var srv MCPServer
		if err := readJSON(r, &srv); err != nil {
			writeError(w, 400, "invalid JSON")
			return
		}
		srv.ID = id
		s.mcpManager.Disconnect(id)
		if err := s.store.UpdateMCPServer(&srv); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		updated, _ := s.store.GetMCPServer(id)
		writeJSON(w, 200, updated)
	case http.MethodDelete:
		s.mcpManager.Disconnect(id)
		if err := s.store.DeleteMCPServer(id); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeError(w, 405, "method not allowed")
	}
}
