package spec

import "time"

// Phase represents which section a step belongs to in a spec.
type Phase string

const (
	PhasePreSuite           Phase = "pre-suite"
	PhasePreSuiteValidation Phase = "pre-suite-validation"
	PhasePreTest            Phase = "pre-test"
	PhasePreTestValidation  Phase = "pre-test-validation"
	PhaseSetup              Phase = "setup"
	PhaseSteps              Phase = "steps"
	PhaseVerify             Phase = "verify"
	PhaseCleanup            Phase = "cleanup"
	PhasePostTest           Phase = "post-test"
	PhasePostSuite          Phase = "post-suite"
)

// Metadata holds structured metadata parsed from a spec's ## Metadata section.
type Metadata struct {
	Author   string
	Priority string // Critical, High, Medium, Low
	CaseID   string // Kiwi TCMS / Jira case ID
	Labels   []string
	Timeout  time.Duration
	Group    string   // Group name for group-level hooks (e.g., "with-loki")
	Fixtures []string // Fixture names to load (e.g., ["flowcollector-default", "test-traffic"])
	Status   string   // "draft" or "approved" (empty treated as "approved" for backward compat)
}

// StepDef is a single step parsed from markdown.
type StepDef struct {
	RawText string // Original markdown line text
	Phase   Phase  // Which section this step came from
	Index   int    // 0-based position within its phase
}

// TestSpec is the parsed representation of a test .md file.
type TestSpec struct {
	Name     string
	FilePath string
	Metadata Metadata
	Setup    []StepDef
	Steps    []StepDef
	Verify   []StepDef
	Cleanup  []StepDef
}

// EffectiveStatus returns the spec status, defaulting to "approved" if empty.
func (s *TestSpec) EffectiveStatus() string {
	if s.Metadata.Status == "" {
		return "approved"
	}
	return s.Metadata.Status
}

// IsDraft returns true if the spec is in draft status.
func (s *TestSpec) IsDraft() bool {
	return s.EffectiveStatus() == "draft"
}

// TestName returns the name in openshift-tests-private convention:
// Author:name-Priority-CaseID-Description [Labels]
func (s *TestSpec) TestName() string {
	name := ""
	if s.Metadata.Author != "" {
		name += "Author:" + s.Metadata.Author + "-"
	}
	if s.Metadata.Priority != "" {
		name += s.Metadata.Priority + "-"
	}
	if s.Metadata.CaseID != "" {
		name += s.Metadata.CaseID + "-"
	}
	name += s.Name
	if len(s.Metadata.Labels) > 0 {
		for _, l := range s.Metadata.Labels {
			name += " [" + l + "]"
		}
	}
	return name
}

// SuiteSpec defines shared hooks for all specs in a directory.
// Parsed from _suite.md files.
type SuiteSpec struct {
	Name               string
	FilePath           string
	Metadata           Metadata
	PreSuite           []StepDef // Runs once before all specs
	PreSuiteValidation []StepDef // Conditions that must be true after pre-suite
	PreTest            []StepDef // Runs before each spec
	PreTestValidation  []StepDef // Conditions that must be true after pre-test
	PostTest           []StepDef // Runs after each spec
	PostSuite          []StepDef // Runs once after all specs
}

// GroupSpec defines shared hooks for a subset of specs sharing a group name.
// Parsed from _group_<name>.md files.
type GroupSpec struct {
	Name              string // Group name (e.g., "with-loki")
	FilePath          string
	Metadata          Metadata
	PreTest           []StepDef // Runs before each spec in this group
	PreTestValidation []StepDef // Conditions that must be true after group pre-test
	PostTest          []StepDef // Runs after each spec in this group
}

// Suite holds a fully loaded suite: the suite spec, group specs, test specs,
// and their relationships.
type Suite struct {
	Dir       string               // Directory path
	SuiteSpec *SuiteSpec           // Optional _suite.md
	Groups    map[string]*GroupSpec // group name → GroupSpec
	Tests     []*TestSpec          // All test specs in the suite
}
