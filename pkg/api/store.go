package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cacheutil "github.com/sdt-project/sdt/pkg/cache"
	"github.com/sdt-project/sdt/pkg/config"
	"github.com/sdt-project/sdt/pkg/fixture"
	"github.com/sdt-project/sdt/pkg/spec"
	"github.com/sdt-project/sdt/pkg/tools"
	"gopkg.in/yaml.v3"
)

// ProjectStore serves SDT project data directly from the project directory.
// Specs = markdown files, Tools = YAML files, MCP = .sdt.yaml config.
// Only execution state (plans, runs, results) uses JSON files in .sdt/tcms/.
type ProjectStore struct {
	projectDir string
	cfg        *config.Config
	mu         sync.RWMutex

	// JSON storage for execution state only
	tcmsDir  string
	counters map[string]*atomic.Int64
}

func NewProjectStore(projectDir string) (*ProjectStore, error) {
	cfg, err := config.Load(projectDir)
	if err != nil {
		cfg = &config.Config{Project: filepath.Base(projectDir)}
	}
	if cfg.Project == "" {
		cfg.Project = filepath.Base(projectDir)
	}
	if cfg.SpecsDir == "" {
		cfg.SpecsDir = "sdt/specs"
	}
	if cfg.ToolsDir == "" {
		cfg.ToolsDir = "sdt/tools"
	}

	tcmsDir := filepath.Join(projectDir, ".sdt", "tcms")
	for _, d := range []string{"plans", "plan-cases", "runs", "results", "step-results",
		"tool-runs", "tool-run-logs", "executions", "execution-logs"} {
		if err := os.MkdirAll(filepath.Join(tcmsDir, d), 0755); err != nil {
			return nil, fmt.Errorf("creating %s dir: %w", d, err)
		}
	}

	ps := &ProjectStore{
		projectDir: projectDir,
		cfg:        cfg,
		tcmsDir:    tcmsDir,
		counters:   make(map[string]*atomic.Int64),
	}

	for _, d := range []string{"plans", "plan-cases", "runs", "results", "step-results",
		"tool-runs", "tool-run-logs", "executions", "execution-logs"} {
		ps.counters[d] = &atomic.Int64{}
		ps.initCounter(d)
	}

	return ps, nil
}

func (ps *ProjectStore) Config() *config.Config {
	return ps.cfg
}

func (ps *ProjectStore) specsDir() string {
	return filepath.Join(ps.projectDir, ps.cfg.SpecsDir)
}

// SpecsDir returns the absolute path to the specs directory.
func (ps *ProjectStore) SpecsDir() string {
	return ps.specsDir()
}

func (ps *ProjectStore) toolsDir() string {
	return filepath.Join(ps.projectDir, ps.cfg.ToolsDir)
}

func (ps *ProjectStore) fixturesDir() string {
	if ps.cfg.FixturesDir != "" {
		return filepath.Join(ps.projectDir, ps.cfg.FixturesDir)
	}
	return filepath.Join(ps.projectDir, "sdt", "fixtures")
}

// --- JSON helpers for execution state ---

func (ps *ProjectStore) initCounter(entityType string) {
	dir := filepath.Join(ps.tcmsDir, entityType)
	entries, _ := os.ReadDir(dir)
	var maxID int64
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if id, err := strconv.ParseInt(name, 10, 64); err == nil && id > maxID {
			maxID = id
		}
	}
	ps.counters[entityType].Store(maxID)
}

func (ps *ProjectStore) nextID(entityType string) int64 {
	return ps.counters[entityType].Add(1)
}

func (ps *ProjectStore) jsonPath(entityType string, id int64) string {
	return filepath.Join(ps.tcmsDir, entityType, fmt.Sprintf("%d.json", id))
}

func (ps *ProjectStore) saveJSON(entityType string, id int64, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.jsonPath(entityType, id), data, 0644)
}

func (ps *ProjectStore) loadJSON(entityType string, id int64, v interface{}) error {
	data, err := os.ReadFile(ps.jsonPath(entityType, id))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (ps *ProjectStore) removeJSON(entityType string, id int64) error {
	return os.Remove(ps.jsonPath(entityType, id))
}

func (ps *ProjectStore) listJSONIDs(entityType string) []int64 {
	dir := filepath.Join(ps.tcmsDir, entityType)
	entries, _ := os.ReadDir(dir)
	var ids []int64
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if id, err := strconv.ParseInt(name, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// === Project (from .sdt.yaml) ===

func (ps *ProjectStore) GetProject() *Project {
	return &Project{
		ID:          1,
		Name:        ps.cfg.Project,
		Description: ps.cfg.Description,
	}
}

func (ps *ProjectStore) ListProjects() []Project {
	return []Project{*ps.GetProject()}
}

func (ps *ProjectStore) UpdateProject(p *Project) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.cfg.Project = p.Name
	ps.cfg.Description = p.Description
	return ps.saveConfig()
}

// === Test Cases (from spec markdown files) ===

func (ps *ProjectStore) ListCases(search string) ([]TestCase, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	dir := ps.specsDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var allCases []TestCase
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || strings.HasPrefix(d.Name(), "_") {
			return nil
		}
		tc, parseErr := ps.specToCase(path)
		if parseErr != nil {
			return nil
		}
		if search != "" {
			sl := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(tc.Title), sl) &&
				!strings.Contains(strings.ToLower(tc.Description), sl) {
				return nil
			}
		}
		allCases = append(allCases, *tc)
		return nil
	})
	return allCases, err
}

func (ps *ProjectStore) GetCase(id int64) (*TestCase, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	cases, _ := ps.ListCases("")
	for _, c := range cases {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, nil
}

func (ps *ProjectStore) GetCaseByPath(relPath string) (*TestCase, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.specToCase(filepath.Join(ps.specsDir(), relPath))
}

func (ps *ProjectStore) CreateCase(c *TestCase) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.specsDir()
	os.MkdirAll(dir, 0755)

	filename := strings.ToLower(strings.ReplaceAll(c.Title, " ", "_")) + ".md"
	path := filepath.Join(dir, filename)

	content := ps.caseToMarkdown(c)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}

	parsed, err := ps.specToCase(path)
	if err != nil {
		return err
	}
	*c = *parsed
	return nil
}

func (ps *ProjectStore) UpdateCase(c *TestCase) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	cases, _ := ps.listCasesInternal("")
	for _, existing := range cases {
		if existing.ID == c.ID {
			content := ps.caseToMarkdown(c)
			return os.WriteFile(existing.filePath, []byte(content), 0644)
		}
	}
	return fmt.Errorf("case not found")
}

func (ps *ProjectStore) DeleteCase(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	cases, _ := ps.listCasesInternal("")
	for _, c := range cases {
		if c.ID == id {
			return os.Remove(c.filePath)
		}
	}
	return fmt.Errorf("case not found")
}

type internalCase struct {
	TestCase
	filePath string
}

func (ps *ProjectStore) listCasesInternal(search string) ([]internalCase, error) {
	dir := ps.specsDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var allCases []internalCase
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || strings.HasPrefix(d.Name(), "_") {
			return nil
		}
		tc, parseErr := ps.specToCase(path)
		if parseErr != nil {
			return nil
		}
		if search != "" {
			sl := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(tc.Title), sl) &&
				!strings.Contains(strings.ToLower(tc.Description), sl) {
				return nil
			}
		}
		allCases = append(allCases, internalCase{TestCase: *tc, filePath: path})
		return nil
	})
	return allCases, nil
}

func (ps *ProjectStore) specToCase(path string) (*TestCase, error) {
	s, err := spec.ParseTestSpec(path)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(ps.specsDir(), path)
	id := stableID(relPath)

	caseID, _ := strconv.ParseInt(s.Metadata.CaseID, 10, 64)
	if caseID > 0 {
		id = caseID
	}

	status := s.EffectiveStatus()
	if status == "approved" {
		status = "active"
	}

	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	var timeout string
	if s.Metadata.Timeout > 0 {
		timeout = s.Metadata.Timeout.String()
	}

	return &TestCase{
		ID:        id,
		ProjectID: 1,
		Title:     s.Name,
		Setup:     stepsToStrings(s.Setup),
		Steps:     stepsToStrings(s.Steps),
		Verify:    stepsToStrings(s.Verify),
		Cleanup:   stepsToStrings(s.Cleanup),
		Priority:  s.Metadata.Priority,
		Status:    status,
		Author:    s.Metadata.Author,
		Labels:    strings.Join(s.Metadata.Labels, ","),
		Group:     s.Metadata.Group,
		Fixtures:  s.Metadata.Fixtures,
		DependsOn: s.Metadata.DependsOn,
		Timeout:   timeout,
		CaseID:    s.Metadata.CaseID,
		CreatedAt: modTime,
		UpdatedAt: modTime,
	}, nil
}

func stepsToStrings(steps []spec.StepDef) []string {
	if len(steps) == 0 {
		return nil
	}
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.RawText
	}
	return out
}

func (ps *ProjectStore) caseToMarkdown(c *TestCase) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Test: %s\n\n", c.Title))

	b.WriteString("## Metadata\n")
	if c.Author != "" {
		b.WriteString(fmt.Sprintf("- Author: %s\n", c.Author))
	}
	if c.Priority != "" {
		b.WriteString(fmt.Sprintf("- Priority: %s\n", c.Priority))
	}
	status := c.Status
	if status == "active" {
		status = "approved"
	}
	b.WriteString(fmt.Sprintf("- Status: %s\n", status))
	if c.Labels != "" {
		b.WriteString(fmt.Sprintf("- Labels: [%s]\n", c.Labels))
	}
	if c.Timeout != "" {
		b.WriteString(fmt.Sprintf("- Timeout: %s\n", c.Timeout))
	}
	if c.Group != "" {
		b.WriteString(fmt.Sprintf("- Group: %s\n", c.Group))
	}
	if len(c.Fixtures) > 0 {
		b.WriteString(fmt.Sprintf("- Fixtures: [%s]\n", strings.Join(c.Fixtures, ", ")))
	}
	if len(c.DependsOn) > 0 {
		b.WriteString(fmt.Sprintf("- DependsOn: [%s]\n", strings.Join(c.DependsOn, ", ")))
	}
	b.WriteString("\n")

	if len(c.Setup) > 0 {
		b.WriteString("## Setup\n")
		for i, s := range c.Setup {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	if len(c.Steps) > 0 {
		b.WriteString("## Steps\n")
		for i, s := range c.Steps {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	if len(c.Verify) > 0 {
		b.WriteString("## Verify\n")
		for _, s := range c.Verify {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
		b.WriteString("\n")
	}

	if len(c.Cleanup) > 0 {
		b.WriteString("## Cleanup\n")
		for i, s := range c.Cleanup {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
	}

	return b.String()
}

// stableID generates a stable integer ID from a file path.
func stableID(path string) int64 {
	var hash int64
	for _, c := range path {
		hash = hash*31 + int64(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash%100000 + 1
}

// === Tools (from YAML files in toolsDir) ===

func (ps *ProjectStore) ListTools(search string) ([]Tool, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	dir := ps.toolsDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var out []Tool
	sl := strings.ToLower(search)
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		t, err := ps.loadToolFromYAML(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(t.Name), sl) &&
			!strings.Contains(strings.ToLower(t.Description), sl) {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}

func (ps *ProjectStore) ListApprovedTools() ([]Tool, error) {
	all, err := ps.ListTools("")
	if err != nil {
		return nil, err
	}
	var out []Tool
	for _, t := range all {
		if t.Status == "approved" {
			out = append(out, t)
		}
	}
	return out, nil
}

func (ps *ProjectStore) GetTool(id int64) (*Tool, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	dir := ps.toolsDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		t, err := ps.loadToolFromYAML(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}

func (ps *ProjectStore) CreateTool(t *Tool) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.toolsDir()
	os.MkdirAll(dir, 0755)

	def := ps.toolToYAMLDef(t)
	filename := strings.ToLower(strings.ReplaceAll(t.Name, " ", "_")) + ".yaml"
	path := filepath.Join(dir, filename)

	if err := tools.SaveCustomToolDef(path, def); err != nil {
		return err
	}

	loaded, _ := ps.loadToolFromYAML(path)
	if loaded != nil {
		*t = *loaded
	}
	return nil
}

func (ps *ProjectStore) UpdateTool(t *Tool) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.toolsDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		existing, err := ps.loadToolFromYAML(path)
		if err != nil || existing.ID != t.ID {
			continue
		}
		def := ps.toolToYAMLDef(t)
		return tools.SaveCustomToolDef(path, def)
	}
	return fmt.Errorf("tool not found")
}

func (ps *ProjectStore) DeleteTool(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.toolsDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		t, err := ps.loadToolFromYAML(path)
		if err != nil || t.ID != id {
			continue
		}
		return os.Remove(path)
	}
	return fmt.Errorf("tool not found")
}

func (ps *ProjectStore) UpdateToolStatus(id int64, status string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.toolsDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		def, err := tools.LoadCustomToolDef(path)
		if err != nil {
			continue
		}
		if stableID(e.Name()) != id {
			continue
		}
		def.Status = status
		return tools.SaveCustomToolDef(path, def)
	}
	return fmt.Errorf("tool not found")
}

func (ps *ProjectStore) loadToolFromYAML(path string) (*Tool, error) {
	def, err := tools.LoadCustomToolDef(path)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(path)
	id := stableID(filename)

	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	inputParams := make(map[string]ToolParam)
	for k, v := range def.Input {
		inputParams[k] = ToolParam{
			Type:        v.Type,
			Description: v.Description,
			Required:    v.Required,
			Default:     v.Default,
		}
	}

	// Parse command into command + args
	parts := strings.Fields(def.Command)
	command := def.Command
	var args []string
	if len(parts) > 1 {
		command = parts[0]
		args = parts[1:]
	}

	return &Tool{
		ID:          id,
		ProjectID:   1,
		Name:        def.Name,
		Description: def.Description,
		Command:     command,
		Args:        args,
		Env:         map[string]string{},
		InputParams: inputParams,
		Category:    def.Category,
		Status:      def.Status,
		Author:      "",
		CreatedAt:   modTime,
		UpdatedAt:   modTime,
	}, nil
}

func (ps *ProjectStore) toolToYAMLDef(t *Tool) *tools.CustomToolDef {
	input := make(map[string]tools.CustomToolParam)
	for k, v := range t.InputParams {
		input[k] = tools.CustomToolParam{
			Type:        v.Type,
			Description: v.Description,
			Required:    v.Required,
			Default:     v.Default,
		}
	}

	cmd := t.Command
	if len(t.Args) > 0 {
		cmd += " " + strings.Join(t.Args, " ")
	}

	status := t.Status
	if status == "" {
		status = "draft"
	}

	return &tools.CustomToolDef{
		Name:        t.Name,
		Description: t.Description,
		Category:    t.Category,
		Status:      status,
		Input:       input,
		Command:     cmd,
	}
}

// === MCP Servers (from .sdt.yaml) ===

func (ps *ProjectStore) ListMCPServers() ([]MCPServer, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var out []MCPServer
	var idx int64
	for name, cfg := range ps.cfg.MCPServers {
		idx++
		out = append(out, MCPServer{
			ID:        stableID(name),
			ProjectID: 1,
			Name:      name,
			Command:   cfg.Command,
			Args:      cfg.Args,
			Env:       cfg.Env,
			Status:    "configured",
		})
	}
	return out, nil
}

func (ps *ProjectStore) GetMCPServer(id int64) (*MCPServer, error) {
	servers, _ := ps.ListMCPServers()
	for _, s := range servers {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, nil
}

func (ps *ProjectStore) CreateMCPServer(s *MCPServer) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.cfg.MCPServers == nil {
		ps.cfg.MCPServers = make(map[string]config.MCPServerConfig)
	}
	ps.cfg.MCPServers[s.Name] = config.MCPServerConfig{
		Command: s.Command,
		Args:    s.Args,
		Env:     s.Env,
	}

	if err := ps.saveConfig(); err != nil {
		return err
	}

	s.ID = stableID(s.Name)
	s.ProjectID = 1
	s.Status = "configured"
	return nil
}

func (ps *ProjectStore) UpdateMCPServer(s *MCPServer) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Find existing by ID
	for name := range ps.cfg.MCPServers {
		if stableID(name) == s.ID {
			delete(ps.cfg.MCPServers, name)
			break
		}
	}
	ps.cfg.MCPServers[s.Name] = config.MCPServerConfig{
		Command: s.Command,
		Args:    s.Args,
		Env:     s.Env,
	}
	return ps.saveConfig()
}

func (ps *ProjectStore) DeleteMCPServer(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for name := range ps.cfg.MCPServers {
		if stableID(name) == id {
			delete(ps.cfg.MCPServers, name)
			return ps.saveConfig()
		}
	}
	return fmt.Errorf("server not found")
}

func (ps *ProjectStore) saveConfig() error {
	data, err := yaml.Marshal(&ps.cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ps.projectDir, ".sdt.yaml"), data, 0644)
}

// === Suite (from _suite.md) ===

func (ps *ProjectStore) GetSuite() (*SuiteInfo, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	suite, err := spec.LoadSuite(ps.specsDir())
	if err != nil {
		return nil, err
	}
	if suite == nil || suite.SuiteSpec == nil {
		return nil, nil
	}

	s := suite.SuiteSpec
	var timeout string
	if s.Metadata.Timeout > 0 {
		timeout = s.Metadata.Timeout.String()
	}

	return &SuiteInfo{
		Name:               s.Name,
		FilePath:           s.FilePath,
		Timeout:            timeout,
		PreSuite:           stepsToStrings(s.PreSuite),
		PreSuiteValidation: stepsToStrings(s.PreSuiteValidation),
		PreTest:            stepsToStrings(s.PreTest),
		PreTestValidation:  stepsToStrings(s.PreTestValidation),
		PostTest:           stepsToStrings(s.PostTest),
		PostSuite:          stepsToStrings(s.PostSuite),
	}, nil
}

func (ps *ProjectStore) SaveSuite(info *SuiteInfo) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.specsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content := ps.suiteToMarkdown(info)
	path := filepath.Join(dir, "_suite.md")
	return os.WriteFile(path, []byte(content), 0644)
}

func (ps *ProjectStore) suiteToMarkdown(info *SuiteInfo) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Suite: %s\n\n", info.Name))

	// Metadata section
	b.WriteString("## Metadata\n")
	if info.Timeout != "" {
		b.WriteString(fmt.Sprintf("- Timeout: %s\n", info.Timeout))
	}
	b.WriteString("\n")

	// Pre-Suite section
	if len(info.PreSuite) > 0 {
		b.WriteString("## Pre-Suite\n")
		for i, s := range info.PreSuite {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Pre-Suite Validation section
	if len(info.PreSuiteValidation) > 0 {
		b.WriteString("## Pre-Suite Validation\n")
		for i, s := range info.PreSuiteValidation {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Pre-Test section
	if len(info.PreTest) > 0 {
		b.WriteString("## Pre-Test\n")
		for i, s := range info.PreTest {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Pre-Test Validation section
	if len(info.PreTestValidation) > 0 {
		b.WriteString("## Pre-Test Validation\n")
		for i, s := range info.PreTestValidation {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Post-Test section
	if len(info.PostTest) > 0 {
		b.WriteString("## Post-Test\n")
		for i, s := range info.PostTest {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Post-Suite section
	if len(info.PostSuite) > 0 {
		b.WriteString("## Post-Suite\n")
		for i, s := range info.PostSuite {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
	}

	return b.String()
}

// === Groups (from _group_*.md) ===

func (ps *ProjectStore) ListGroups() ([]GroupInfo, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	suite, err := spec.LoadSuite(ps.specsDir())
	if err != nil {
		return nil, err
	}
	if suite == nil {
		return nil, nil
	}

	specsByGroup := make(map[string][]string)
	for _, t := range suite.Tests {
		if t.Metadata.Group != "" {
			specsByGroup[t.Metadata.Group] = append(specsByGroup[t.Metadata.Group], t.Name)
		}
	}

	var out []GroupInfo
	for name, g := range suite.Groups {
		var timeout string
		if g.Metadata.Timeout > 0 {
			timeout = g.Metadata.Timeout.String()
		}
		out = append(out, GroupInfo{
			Name:              name,
			FilePath:          g.FilePath,
			Timeout:           timeout,
			PreTest:           stepsToStrings(g.PreTest),
			PreTestValidation: stepsToStrings(g.PreTestValidation),
			PostTest:          stepsToStrings(g.PostTest),
			Specs:             specsByGroup[name],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (ps *ProjectStore) GetGroup(name string) (*GroupInfo, error) {
	groups, err := ps.ListGroups()
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.Name == name {
			return &g, nil
		}
	}
	return nil, nil
}

func (ps *ProjectStore) CreateGroup(info *GroupInfo) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.specsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("_group_%s.md", strings.ToLower(strings.ReplaceAll(info.Name, " ", "_")))
	path := filepath.Join(dir, filename)

	content := ps.groupToMarkdown(info)
	return os.WriteFile(path, []byte(content), 0644)
}

func (ps *ProjectStore) UpdateGroup(oldName string, info *GroupInfo) error {
	dir := ps.specsDir()

	oldPath := ps.findGroupFile(oldName)
	if oldPath == "" {
		oldPath = filepath.Join(dir, fmt.Sprintf("_group_%s.md", strings.ToLower(strings.ReplaceAll(oldName, " ", "_"))))
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if oldName != info.Name {
		os.Remove(oldPath)
		filename := fmt.Sprintf("_group_%s.md", strings.ToLower(strings.ReplaceAll(info.Name, " ", "_")))
		oldPath = filepath.Join(dir, filename)
	}

	content := ps.groupToMarkdown(info)
	return os.WriteFile(oldPath, []byte(content), 0644)
}

func (ps *ProjectStore) DeleteGroup(name string) error {
	dir := ps.specsDir()

	path := ps.findGroupFile(name)
	if path == "" {
		path = filepath.Join(dir, fmt.Sprintf("_group_%s.md", strings.ToLower(strings.ReplaceAll(name, " ", "_"))))
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	return os.Remove(path)
}

func (ps *ProjectStore) findGroupFile(name string) string {
	groups, err := ps.ListGroups()
	if err != nil {
		return ""
	}
	for _, g := range groups {
		if g.Name == name {
			return g.FilePath
		}
	}
	return ""
}

func (ps *ProjectStore) groupToMarkdown(info *GroupInfo) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Group: %s\n\n", info.Name))

	// Metadata section
	b.WriteString("## Metadata\n")
	if info.Timeout != "" {
		b.WriteString(fmt.Sprintf("- Timeout: %s\n", info.Timeout))
	}
	b.WriteString("\n")

	// Pre-Test section
	if len(info.PreTest) > 0 {
		b.WriteString("## Pre-Test\n")
		for i, s := range info.PreTest {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Pre-Test Validation section
	if len(info.PreTestValidation) > 0 {
		b.WriteString("## Pre-Test Validation\n")
		for i, s := range info.PreTestValidation {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	// Post-Test section
	if len(info.PostTest) > 0 {
		b.WriteString("## Post-Test\n")
		for i, s := range info.PostTest {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
	}

	return b.String()
}

// === Fixtures (from fixturesDir YAML files) ===

func (ps *ProjectStore) ListFixtures() ([]Fixture, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	dir := ps.fixturesDir()
	registry, err := fixture.LoadDirRecursive(dir)
	if err != nil {
		return nil, err
	}

	var out []Fixture
	for _, name := range registry.List() {
		def := registry.Get(name)
		out = append(out, defToFixture(def))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (ps *ProjectStore) GetFixture(name string) (*Fixture, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	dir := ps.fixturesDir()
	registry, err := fixture.LoadDirRecursive(dir)
	if err != nil {
		return nil, err
	}

	def := registry.Get(name)
	if def == nil {
		return nil, nil
	}
	f := defToFixture(def)
	return &f, nil
}

func defToFixture(def *fixture.Definition) Fixture {
	status := def.EffectiveStatus()
	if status == "approved" {
		status = "active"
	}
	return Fixture{
		Name:        def.Name,
		Description: def.Description,
		Status:      status,
		Templates:   def.TemplatePaths(),
		Parameters:  def.Parameters,
		Lifecycle: FixtureLifecycle{
			Create:  def.Lifecycle.Create,
			Ready:   def.Lifecycle.Ready,
			Cleanup: def.Lifecycle.Cleanup,
		},
	}
}

func (ps *ProjectStore) CreateFixture(f *Fixture) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.fixturesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	def := fixtureToDefinition(f)
	filename := strings.ToLower(strings.ReplaceAll(f.Name, " ", "_")) + ".yaml"
	path := filepath.Join(dir, filename)

	data, err := yaml.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (ps *ProjectStore) UpdateFixture(oldName string, f *Fixture) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.fixturesDir()
	srcPath, srcDefs := ps.findFixtureInDir(dir, oldName)

	newDef := fixtureToDefinition(f)

	if srcPath != "" && len(srcDefs) > 1 {
		// Multi-doc file: rewrite with updated definition in place
		var updated []*fixture.Definition
		for _, d := range srcDefs {
			if d.Name == oldName {
				updated = append(updated, newDef)
			} else {
				updated = append(updated, d)
			}
		}
		return ps.writeMultiDocFixture(srcPath, updated)
	}

	// Single-doc or not found: remove old, write new standalone file
	if srcPath != "" {
		os.Remove(srcPath)
	}

	filename := strings.ToLower(strings.ReplaceAll(f.Name, " ", "_")) + ".yaml"
	path := filepath.Join(dir, filename)
	data, err := yaml.Marshal(newDef)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (ps *ProjectStore) DeleteFixture(name string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	dir := ps.fixturesDir()
	srcPath, srcDefs := ps.findFixtureInDir(dir, name)

	if srcPath == "" {
		filename := strings.ToLower(strings.ReplaceAll(name, " ", "_")) + ".yaml"
		return os.Remove(filepath.Join(dir, filename))
	}

	if len(srcDefs) > 1 {
		// Multi-doc: rewrite without the deleted fixture
		var remaining []*fixture.Definition
		for _, d := range srcDefs {
			if d.Name != name {
				remaining = append(remaining, d)
			}
		}
		return ps.writeMultiDocFixture(srcPath, remaining)
	}

	return os.Remove(srcPath)
}

func (ps *ProjectStore) findFixtureInDir(dir, name string) (string, []*fixture.Definition) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fname := entry.Name()
		if !strings.HasSuffix(fname, ".yaml") && !strings.HasSuffix(fname, ".yml") {
			continue
		}
		path := filepath.Join(dir, fname)
		defs, err := fixture.LoadFile(path)
		if err != nil {
			continue
		}
		for _, def := range defs {
			if def.Name == name {
				return path, defs
			}
		}
	}
	return "", nil
}

func (ps *ProjectStore) writeMultiDocFixture(path string, defs []*fixture.Definition) error {
	var buf strings.Builder
	for i, def := range defs {
		if i > 0 {
			buf.WriteString("---\n")
		}
		data, err := yaml.Marshal(def)
		if err != nil {
			return err
		}
		buf.Write(data)
	}
	return os.WriteFile(path, []byte(buf.String()), 0644)
}

func fixtureToDefinition(f *Fixture) *fixture.Definition {
	status := f.Status
	if status == "active" {
		status = "approved"
	}

	templates := f.Templates
	if len(templates) == 0 && len(f.Templates) > 0 {
		templates = f.Templates
	}

	def := &fixture.Definition{
		Name:        f.Name,
		Description: f.Description,
		Status:      status,
		Parameters:  f.Parameters,
		Lifecycle: fixture.Lifecycle{
			Create:  f.Lifecycle.Create,
			Ready:   f.Lifecycle.Ready,
			Cleanup: f.Lifecycle.Cleanup,
		},
	}

	// Use Template if only one, otherwise Templates if multiple
	if len(templates) == 1 {
		def.Template = templates[0]
	} else if len(templates) > 1 {
		def.Templates = templates
	}

	return def
}

// === Test Plans (JSON in .sdt/tcms/) ===

func (ps *ProjectStore) ListPlans() ([]TestPlan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	ids := ps.listJSONIDs("plans")
	var out []TestPlan
	for _, id := range ids {
		var p TestPlan
		if err := ps.loadJSON("plans", id, &p); err == nil {
			p.CaseCount = ps.countPlanCases(p.ID)
			out = append(out, p)
		}
	}
	return out, nil
}

func (ps *ProjectStore) GetPlan(id int64) (*TestPlan, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var p TestPlan
	if err := ps.loadJSON("plans", id, &p); err != nil {
		return nil, nil
	}
	p.CaseCount = ps.countPlanCases(p.ID)
	return &p, nil
}

func (ps *ProjectStore) CreatePlan(p *TestPlan) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p.ID = ps.nextID("plans")
	p.ProjectID = 1
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	return ps.saveJSON("plans", p.ID, p)
}

func (ps *ProjectStore) UpdatePlan(p *TestPlan) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var existing TestPlan
	if err := ps.loadJSON("plans", p.ID, &existing); err != nil {
		return fmt.Errorf("plan not found")
	}
	p.CreatedAt = existing.CreatedAt
	p.ProjectID = 1
	p.UpdatedAt = time.Now()
	return ps.saveJSON("plans", p.ID, p)
}

func (ps *ProjectStore) DeletePlan(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.removeJSON("plan-cases", id)
	return ps.removeJSON("plans", id)
}

type planCaseMapping struct {
	CaseIDs []int64 `json:"case_ids"`
}

func (ps *ProjectStore) countPlanCases(planID int64) int {
	var m planCaseMapping
	if err := ps.loadJSON("plan-cases", planID, &m); err != nil {
		return 0
	}
	return len(m.CaseIDs)
}

func (ps *ProjectStore) GetPlanCases(planID int64) ([]TestCase, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var m planCaseMapping
	if err := ps.loadJSON("plan-cases", planID, &m); err != nil {
		return nil, nil
	}
	var out []TestCase
	allCases, _ := ps.ListCases("")
	caseMap := make(map[int64]TestCase)
	for _, c := range allCases {
		caseMap[c.ID] = c
	}
	for _, cid := range m.CaseIDs {
		if c, ok := caseMap[cid]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (ps *ProjectStore) SetPlanCases(planID int64, caseIDs []int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.saveJSON("plan-cases", planID, &planCaseMapping{CaseIDs: caseIDs})
}

func (ps *ProjectStore) RemoveCaseFromPlan(planID, caseID int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var m planCaseMapping
	ps.loadJSON("plan-cases", planID, &m)
	var filtered []int64
	for _, id := range m.CaseIDs {
		if id != caseID {
			filtered = append(filtered, id)
		}
	}
	return ps.saveJSON("plan-cases", planID, &planCaseMapping{CaseIDs: filtered})
}

// === Test Runs (JSON in .sdt/tcms/) ===

func (ps *ProjectStore) ListRuns(planID int64) ([]TestRun, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	ids := ps.listJSONIDs("runs")
	var out []TestRun
	for _, id := range ids {
		var r TestRun
		if err := ps.loadJSON("runs", id, &r); err != nil || r.PlanID != planID {
			continue
		}
		ps.computeRunCounts(&r)
		out = append(out, r)
	}
	return out, nil
}

func (ps *ProjectStore) ListAllRuns() ([]TestRun, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	ids := ps.listJSONIDs("runs")
	planNames := make(map[int64]string)
	for _, pid := range ps.listJSONIDs("plans") {
		var p TestPlan
		if err := ps.loadJSON("plans", pid, &p); err == nil {
			planNames[p.ID] = p.Name
		}
	}
	var out []TestRun
	for _, id := range ids {
		var r TestRun
		if err := ps.loadJSON("runs", id, &r); err != nil {
			continue
		}
		r.PlanName = planNames[r.PlanID]
		ps.computeRunCounts(&r)
		out = append(out, r)
	}
	return out, nil
}

func (ps *ProjectStore) GetRun(id int64) (*TestRun, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var r TestRun
	if err := ps.loadJSON("runs", id, &r); err != nil {
		return nil, nil
	}
	var p TestPlan
	if err := ps.loadJSON("plans", r.PlanID, &p); err == nil {
		r.PlanName = p.Name
	}
	ps.computeRunCounts(&r)
	return &r, nil
}

func (ps *ProjectStore) CreateRun(r *TestRun) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	r.ID = ps.nextID("runs")
	r.CreatedAt = time.Now()
	r.Status = "not_started"
	if err := ps.saveJSON("runs", r.ID, r); err != nil {
		return err
	}
	var m planCaseMapping
	if err := ps.loadJSON("plan-cases", r.PlanID, &m); err == nil {
		for _, caseID := range m.CaseIDs {
			result := &TestResult{
				ID:     ps.nextID("results"),
				RunID:  r.ID,
				CaseID: caseID,
				Status: "untested",
			}
			ps.saveJSON("results", result.ID, result)
		}
	}
	return nil
}

func (ps *ProjectStore) DeleteRun(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, rid := range ps.listJSONIDs("results") {
		var r TestResult
		if err := ps.loadJSON("results", rid, &r); err == nil && r.RunID == id {
			ps.removeJSON("results", rid)
			ps.removeJSON("step-results", rid)
		}
	}
	return ps.removeJSON("runs", id)
}

func (ps *ProjectStore) StartRun(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var r TestRun
	if err := ps.loadJSON("runs", id, &r); err != nil {
		return fmt.Errorf("run not found")
	}
	now := time.Now()
	r.StartedAt = &now
	r.Status = "in_progress"
	return ps.saveJSON("runs", id, &r)
}

func (ps *ProjectStore) CompleteRun(id int64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var r TestRun
	if err := ps.loadJSON("runs", id, &r); err != nil {
		return fmt.Errorf("run not found")
	}
	now := time.Now()
	r.CompletedAt = &now
	r.Status = "completed"
	return ps.saveJSON("runs", id, &r)
}

func (ps *ProjectStore) computeRunCounts(r *TestRun) {
	for _, rid := range ps.listJSONIDs("results") {
		var res TestResult
		if err := ps.loadJSON("results", rid, &res); err != nil || res.RunID != r.ID {
			continue
		}
		r.Total++
		switch res.Status {
		case "passed":
			r.Passed++
		case "failed":
			r.Failed++
		case "blocked":
			r.Blocked++
		case "skipped":
			r.Skipped++
		}
	}
}

// === Test Results (JSON in .sdt/tcms/) ===

func (ps *ProjectStore) ListResults(runID int64) ([]TestResult, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var out []TestResult
	for _, id := range ps.listJSONIDs("results") {
		var r TestResult
		if err := ps.loadJSON("results", id, &r); err != nil || r.RunID != runID {
			continue
		}
		c, _ := ps.GetCase(r.CaseID)
		r.Case = c
		var steps []TestStepResult
		ps.loadJSON("step-results", r.ID, &steps)
		r.StepResults = steps
		out = append(out, r)
	}
	return out, nil
}

func (ps *ProjectStore) GetResult(id int64) (*TestResult, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var r TestResult
	if err := ps.loadJSON("results", id, &r); err != nil {
		return nil, nil
	}
	c, _ := ps.GetCase(r.CaseID)
	r.Case = c
	var steps []TestStepResult
	ps.loadJSON("step-results", r.ID, &steps)
	r.StepResults = steps
	return &r, nil
}

func (ps *ProjectStore) UpdateResult(r *TestResult) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	now := time.Now()
	r.ExecutedAt = &now
	return ps.saveJSON("results", r.ID, r)
}

func (ps *ProjectStore) SaveStepResults(resultID int64, steps []TestStepResult) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for i := range steps {
		steps[i].ResultID = resultID
		if steps[i].ID == 0 {
			steps[i].ID = int64(i + 1)
		}
	}
	return ps.saveJSON("step-results", resultID, &steps)
}

// === Tool Runs (JSON in .sdt/tcms/) ===

func (ps *ProjectStore) CreateToolRun(r *ToolRun) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	r.ID = ps.nextID("tool-runs")
	r.CreatedAt = time.Now()
	return ps.saveJSON("tool-runs", r.ID, r)
}

func (ps *ProjectStore) UpdateToolRun(r *ToolRun) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.saveJSON("tool-runs", r.ID, r)
}

func (ps *ProjectStore) GetToolRun(id int64) (*ToolRun, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var r ToolRun
	if err := ps.loadJSON("tool-runs", id, &r); err != nil {
		return nil, nil
	}
	return &r, nil
}

func (ps *ProjectStore) ListToolRuns(toolID int64) ([]ToolRun, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var out []ToolRun
	for _, id := range ps.listJSONIDs("tool-runs") {
		var r ToolRun
		if err := ps.loadJSON("tool-runs", id, &r); err != nil || r.ToolID != toolID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

type toolRunLogStore struct {
	Logs []ToolRunLog `json:"logs"`
}

func (ps *ProjectStore) AppendToolLog(l *ToolRunLog) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var store toolRunLogStore
	ps.loadJSON("tool-run-logs", l.RunID, &store)
	l.ID = int64(len(store.Logs) + 1)
	l.Timestamp = time.Now()
	store.Logs = append(store.Logs, *l)
	return ps.saveJSON("tool-run-logs", l.RunID, &store)
}

func (ps *ProjectStore) GetToolLogsSince(runID int64, afterID int64) ([]ToolRunLog, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var store toolRunLogStore
	if err := ps.loadJSON("tool-run-logs", runID, &store); err != nil {
		return nil, nil
	}
	var out []ToolRunLog
	for _, l := range store.Logs {
		if l.ID > afterID {
			out = append(out, l)
		}
	}
	return out, nil
}

// === Executions (JSON in .sdt/tcms/) ===

func (ps *ProjectStore) CreateExecution(e *Execution) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	e.ID = ps.nextID("executions")
	e.CreatedAt = time.Now()
	return ps.saveJSON("executions", e.ID, e)
}

func (ps *ProjectStore) UpdateExecution(e *Execution) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.saveJSON("executions", e.ID, e)
}

func (ps *ProjectStore) GetExecution(id int64) (*Execution, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var e Execution
	if err := ps.loadJSON("executions", id, &e); err != nil {
		return nil, fmt.Errorf("execution not found")
	}
	return &e, nil
}

func (ps *ProjectStore) ListExecutions(caseID int64) ([]Execution, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var out []Execution
	for _, id := range ps.listJSONIDs("executions") {
		var e Execution
		if err := ps.loadJSON("executions", id, &e); err != nil || e.CaseID != caseID {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

type executionLogStore struct {
	Logs []ExecutionLog `json:"logs"`
}

func (ps *ProjectStore) AppendLog(l *ExecutionLog) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var store executionLogStore
	ps.loadJSON("execution-logs", l.ExecutionID, &store)
	l.ID = int64(len(store.Logs) + 1)
	l.Timestamp = time.Now()
	store.Logs = append(store.Logs, *l)
	return ps.saveJSON("execution-logs", l.ExecutionID, &store)
}

func (ps *ProjectStore) GetLogsSince(executionID int64, afterID int64) ([]ExecutionLog, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var store executionLogStore
	if err := ps.loadJSON("execution-logs", executionID, &store); err != nil {
		return nil, nil
	}
	var out []ExecutionLog
	for _, l := range store.Logs {
		if l.ID > afterID {
			out = append(out, l)
		}
	}
	return out, nil
}

// === Dashboard ===

func (ps *ProjectStore) GetDashboardStats() (*DashboardStats, error) {
	cases, _ := ps.ListCases("")
	plans, _ := ps.ListPlans()
	runs, _ := ps.ListAllRuns()
	activeRuns := 0
	for _, r := range runs {
		if r.Status == "in_progress" {
			activeRuns++
		}
	}
	return &DashboardStats{
		Projects:   1,
		TotalCases: len(cases),
		TotalPlans: len(plans),
		TotalRuns:  len(runs),
		ActiveRuns: activeRuns,
	}, nil
}

func (ps *ProjectStore) GetTrends(days int) ([]TrendPoint, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	dateCounts := make(map[string]*TrendPoint)
	for _, id := range ps.listJSONIDs("results") {
		var r TestResult
		if err := ps.loadJSON("results", id, &r); err != nil || r.ExecutedAt == nil {
			continue
		}
		dateStr := r.ExecutedAt.Format("2006-01-02")
		tp, ok := dateCounts[dateStr]
		if !ok {
			tp = &TrendPoint{Date: dateStr}
			dateCounts[dateStr] = tp
		}
		tp.Total++
		switch r.Status {
		case "passed":
			tp.Passed++
		case "failed":
			tp.Failed++
		case "blocked":
			tp.Blocked++
		case "skipped":
			tp.Skipped++
		}
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var out []TrendPoint
	for _, tp := range dateCounts {
		if tp.Date >= cutoff {
			out = append(out, *tp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// --- Cache ---

type CachePlanStep struct {
	Description    string                 `json:"description"`
	ToolName       string                 `json:"tool_name"`
	Parameters     map[string]interface{} `json:"parameters"`
	ExpectedResult string                 `json:"expected_result"`
	Validation     interface{}            `json:"validation"`
	OnFailure      string                 `json:"on_failure"`
}

type CachePlanPhase struct {
	Name  string          `json:"name"`
	Steps []CachePlanStep `json:"steps"`
}

type CacheFixturePlan struct {
	Name       string                 `json:"name"`
	Template   string                 `json:"template"`
	Parameters map[string]interface{} `json:"parameters"`
	Create     []CachePlanStep        `json:"create"`
	ReadyCheck []CachePlanStep        `json:"ready_check"`
	Cleanup    []CachePlanStep        `json:"cleanup"`
}

type CachedPlan struct {
	SpecHash  string             `json:"spec_hash"`
	SpecName  string             `json:"spec_name"`
	CreatedAt string             `json:"created_at"`
	Model     string             `json:"model"`
	Phases    []CachePlanPhase   `json:"phases"`
	Fixtures  []CacheFixturePlan `json:"fixtures"`
	FileSize  int64              `json:"file_size"`
	FileName  string             `json:"file_name"`
}

type CacheStepResult struct {
	Description string `json:"description"`
	ToolName    string `json:"tool_name"`
	Status      string `json:"status"`
	Output      string `json:"output"`
	Error       string `json:"error,omitempty"`
	Duration    int64  `json:"duration"`
}

type CachePhaseResult struct {
	Phase       string            `json:"phase"`
	Status      string            `json:"status"`
	StepResults []CacheStepResult `json:"step_results"`
	Error       string            `json:"error,omitempty"`
}

type CacheFixtureResult struct {
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	CreateResult  []CacheStepResult `json:"create"`
	ReadyResult   []CacheStepResult `json:"ready_check"`
	CleanupResult []CacheStepResult `json:"cleanup"`
	Error         string            `json:"error,omitempty"`
}

type CachedResult struct {
	SpecHash       string               `json:"spec_hash"`
	SpecName       string               `json:"spec_name"`
	RunID          string               `json:"run_id"`
	Status         string               `json:"status"`
	StartTime      string               `json:"start_time"`
	EndTime        string               `json:"end_time"`
	Duration       int64                `json:"duration"`
	Error          string               `json:"error"`
	Timestamp      string               `json:"timestamp"`
	PhaseResults   []CachePhaseResult   `json:"phase_results"`
	FixtureResults []CacheFixtureResult `json:"fixture_results"`
	CleanupRun     bool                 `json:"cleanup_run"`
	FileSize       int64                `json:"file_size"`
	FileName       string               `json:"file_name"`
}

type CacheSummary struct {
	Plans       []CachedPlan   `json:"plans"`
	Results     []CachedResult `json:"results"`
	TotalSize   int64          `json:"total_size"`
	PlanCount   int            `json:"plan_count"`
	ResultCount int            `json:"result_count"`
}

func (ps *ProjectStore) cacheDir() string {
	return filepath.Join(ps.projectDir, ".sdt", "cache")
}

func (ps *ProjectStore) GetCacheSummary() (*CacheSummary, error) {
	base := ps.cacheDir()
	summary := &CacheSummary{
		Plans:   []CachedPlan{},
		Results: []CachedResult{},
	}

	plansDir := filepath.Join(base, "plans")
	if entries, err := os.ReadDir(plansDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			path := filepath.Join(plansDir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			info, _ := e.Info()

			var cp CachedPlan
			if err := json.Unmarshal(data, &cp); err != nil {
				continue
			}
			cp.FileName = e.Name()
			if cp.SpecHash == "" {
				cp.SpecHash = strings.TrimSuffix(e.Name(), ".json")
			}
			if info != nil {
				cp.FileSize = info.Size()
				summary.TotalSize += info.Size()
			}
			if cp.Phases == nil {
				cp.Phases = []CachePlanPhase{}
			}
			if cp.Fixtures == nil {
				cp.Fixtures = []CacheFixturePlan{}
			}
			summary.Plans = append(summary.Plans, cp)
		}
	}
	summary.PlanCount = len(summary.Plans)

	resultsDir := filepath.Join(base, "results")
	if hashDirs, err := os.ReadDir(resultsDir); err == nil {
		for _, hd := range hashDirs {
			if !hd.IsDir() {
				continue
			}
			specHash := hd.Name()
			rdPath := filepath.Join(resultsDir, specHash)
			files, err := os.ReadDir(rdPath)
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() || strings.HasSuffix(f.Name(), ".meta") || !strings.HasSuffix(f.Name(), ".json") {
					continue
				}
				path := filepath.Join(rdPath, f.Name())
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				info, _ := f.Info()

				var cr CachedResult
				if err := json.Unmarshal(data, &cr); err != nil {
					continue
				}
				cr.SpecHash = specHash
				cr.FileName = f.Name()
				if info != nil {
					cr.FileSize = info.Size()
					summary.TotalSize += info.Size()
				}
				if cr.PhaseResults == nil {
					cr.PhaseResults = []CachePhaseResult{}
				}
				if cr.FixtureResults == nil {
					cr.FixtureResults = []CacheFixtureResult{}
				}

				metaPath := path + ".meta"
				if metaData, err := os.ReadFile(metaPath); err == nil {
					var meta struct {
						RunID     string `json:"run_id"`
						Timestamp string `json:"timestamp"`
					}
					if json.Unmarshal(metaData, &meta) == nil {
						cr.RunID = meta.RunID
						cr.Timestamp = meta.Timestamp
					}
				}
				summary.Results = append(summary.Results, cr)
			}
		}
	}
	summary.ResultCount = len(summary.Results)

	sort.Slice(summary.Plans, func(i, j int) bool {
		return summary.Plans[i].CreatedAt > summary.Plans[j].CreatedAt
	})
	sort.Slice(summary.Results, func(i, j int) bool {
		return summary.Results[i].Timestamp > summary.Results[j].Timestamp
	})

	return summary, nil
}

func (ps *ProjectStore) ClearCache() error {
	return os.RemoveAll(ps.cacheDir())
}

// CaseCache holds cached plans and results for a specific test case/spec.
type CaseCache struct {
	SpecHashes []string       `json:"spec_hashes"`
	Plans      []CachedPlan   `json:"plans"`
	Results    []CachedResult `json:"results"`
}

// GetCacheForCase finds cached plans and results for a specific test case.
// Uses SHA256 of file content as the canonical hash.
func (ps *ProjectStore) GetCacheForCase(caseID int64) (*CaseCache, error) {
	specPath, err := ps.specPathForCase(caseID)
	if err != nil {
		return nil, err
	}
	if specPath == "" {
		return &CaseCache{Plans: []CachedPlan{}, Results: []CachedResult{}}, nil
	}

	specHash := cacheutil.ComputeSpecHashFromFile(specPath)
	hashes := []string{specHash}

	base := ps.cacheDir()
	cc := &CaseCache{
		SpecHashes: hashes,
		Plans:      []CachedPlan{},
		Results:    []CachedResult{},
	}

	// Look up plans by hash
	plansDir := filepath.Join(base, "plans")
	for _, h := range hashes {
		path := filepath.Join(plansDir, h+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cp CachedPlan
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		if cp.SpecHash == "" {
			cp.SpecHash = h
		}
		cp.FileName = h + ".json"
		if info, err := os.Stat(path); err == nil {
			cp.FileSize = info.Size()
		}
		if cp.Phases == nil {
			cp.Phases = []CachePlanPhase{}
		}
		if cp.Fixtures == nil {
			cp.Fixtures = []CacheFixturePlan{}
		}
		cc.Plans = append(cc.Plans, cp)
	}

	// Look up results by hash
	resultsDir := filepath.Join(base, "results")
	for _, h := range hashes {
		rdPath := filepath.Join(resultsDir, h)
		files, err := os.ReadDir(rdPath)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || strings.HasSuffix(f.Name(), ".meta") || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			path := filepath.Join(rdPath, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var cr CachedResult
			if err := json.Unmarshal(data, &cr); err != nil {
				continue
			}
			cr.SpecHash = h
			cr.FileName = f.Name()
			if info, err := os.Stat(path); err == nil {
				cr.FileSize = info.Size()
			}
			if cr.PhaseResults == nil {
				cr.PhaseResults = []CachePhaseResult{}
			}
			if cr.FixtureResults == nil {
				cr.FixtureResults = []CacheFixtureResult{}
			}

			metaPath := path + ".meta"
			if metaData, err := os.ReadFile(metaPath); err == nil {
				var meta struct {
					RunID     string `json:"run_id"`
					Timestamp string `json:"timestamp"`
				}
				if json.Unmarshal(metaData, &meta) == nil {
					cr.RunID = meta.RunID
					cr.Timestamp = meta.Timestamp
				}
			}
			cc.Results = append(cc.Results, cr)
		}
	}

	sort.Slice(cc.Results, func(i, j int) bool {
		return cc.Results[i].Timestamp > cc.Results[j].Timestamp
	})

	return cc, nil
}

// DeleteCacheForCase removes cached plans for a specific test case.
func (ps *ProjectStore) DeleteCacheForCase(caseID int64) error {
	specPath, err := ps.specPathForCase(caseID)
	if err != nil {
		return err
	}
	if specPath == "" {
		return fmt.Errorf("spec file not found for case %d", caseID)
	}

	specHash := cacheutil.ComputeSpecHashFromFile(specPath)
	planPath := filepath.Join(ps.cacheDir(), "plans", specHash+".json")
	if err := os.Remove(planPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing cached plan: %w", err)
	}
	return nil
}

// specPathForCase resolves the on-disk spec file path for a case ID.
func (ps *ProjectStore) specPathForCase(caseID int64) (string, error) {
	cases, err := ps.listCasesInternal("")
	if err != nil {
		return "", err
	}
	for _, c := range cases {
		if c.ID == caseID {
			return c.filePath, nil
		}
	}
	return "", nil
}

func hookHash(phaseName string, steps []string) string {
	return cacheutil.ComputeHookHash(phaseName, steps)
}

func stepsContentHash(title string, steps []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Test: %s\n\n", title))
	b.WriteString("## Metadata\n")
	b.WriteString("- Status: draft\n")
	b.WriteString("- Priority: Medium\n\n")
	b.WriteString("## Steps\n")
	for i, s := range steps {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}
	return cacheutil.ComputeContentHash(b.String())
}

func (ps *ProjectStore) lookupCachePlans(hashes []string) (*CaseCache, error) {
	base := ps.cacheDir()
	cc := &CaseCache{
		SpecHashes: hashes,
		Plans:      []CachedPlan{},
		Results:    []CachedResult{},
	}

	plansDir := filepath.Join(base, "plans")
	for _, h := range hashes {
		path := filepath.Join(plansDir, h+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cp CachedPlan
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		if cp.SpecHash == "" {
			cp.SpecHash = h
		}
		cp.FileName = h + ".json"
		if info, err := os.Stat(path); err == nil {
			cp.FileSize = info.Size()
		}
		if cp.Phases == nil {
			cp.Phases = []CachePlanPhase{}
		}
		if cp.Fixtures == nil {
			cp.Fixtures = []CacheFixturePlan{}
		}
		cc.Plans = append(cc.Plans, cp)
	}

	return cc, nil
}

func (ps *ProjectStore) GetCacheForSuite() (*CaseCache, error) {
	suite, err := ps.GetSuite()
	if err != nil || suite == nil {
		return &CaseCache{Plans: []CachedPlan{}, Results: []CachedResult{}}, nil
	}

	phases := []struct {
		name  string
		title string
		steps []string
	}{
		{"pre-suite", suite.Name + " — Pre-Suite", suite.PreSuite},
		{"suite-pre-test", suite.Name + " — Pre-Test", suite.PreTest},
		{"suite-post-test", suite.Name + " — Post-Test", suite.PostTest},
		{"post-suite", suite.Name + " — Post-Suite", suite.PostSuite},
	}

	var hashes []string
	for _, p := range phases {
		if len(p.steps) > 0 {
			hashes = append(hashes, hookHash(p.name, p.steps))
			hashes = append(hashes, stepsContentHash(p.title, p.steps))
		}
	}

	return ps.lookupCachePlans(hashes)
}

func (ps *ProjectStore) GetCacheForGroup(name string) (*CaseCache, error) {
	group, err := ps.GetGroup(name)
	if err != nil || group == nil {
		return &CaseCache{Plans: []CachedPlan{}, Results: []CachedResult{}}, nil
	}

	phases := []struct {
		phaseName string
		steps     []string
	}{
		{"group-pre-test", group.PreTest},
		{"group-post-test", group.PostTest},
	}

	var hashes []string
	for _, p := range phases {
		if len(p.steps) > 0 {
			hashes = append(hashes, hookHash(p.phaseName, p.steps))
		}
	}

	return ps.lookupCachePlans(hashes)
}
