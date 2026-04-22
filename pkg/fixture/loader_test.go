package fixture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_SingleDocument(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "fixture.yaml")

	fixtureContent := `name: test-fixture
description: A test fixture
template: templates/test.yaml
parameters:
  Key1: value1
  Key2: value2
lifecycle:
  create: Apply the template
  ready: Wait for pod to be ready
  cleanup: Delete the pod
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("expected 1 definition, got %d", len(defs))
	}

	def := defs[0]
	if def.Name != "test-fixture" {
		t.Errorf("expected name 'test-fixture', got '%s'", def.Name)
	}

	if def.Description != "A test fixture" {
		t.Errorf("expected description 'A test fixture', got '%s'", def.Description)
	}

	if def.Template != "templates/test.yaml" {
		t.Errorf("expected template 'templates/test.yaml', got '%s'", def.Template)
	}

	if len(def.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(def.Parameters))
	}

	if def.Parameters["Key1"] != "value1" {
		t.Errorf("expected Key1=value1, got %v", def.Parameters["Key1"])
	}

	if def.Parameters["Key2"] != "value2" {
		t.Errorf("expected Key2=value2, got %v", def.Parameters["Key2"])
	}

	if def.Lifecycle.Create != "Apply the template" {
		t.Errorf("expected lifecycle.create 'Apply the template', got '%s'", def.Lifecycle.Create)
	}

	if def.Lifecycle.Ready != "Wait for pod to be ready" {
		t.Errorf("expected lifecycle.ready 'Wait for pod to be ready', got '%s'", def.Lifecycle.Ready)
	}

	if def.Lifecycle.Cleanup != "Delete the pod" {
		t.Errorf("expected lifecycle.cleanup 'Delete the pod', got '%s'", def.Lifecycle.Cleanup)
	}
}

func TestLoadFile_MultiDocument(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "fixtures.yaml")

	fixtureContent := `name: fixture-one
description: First fixture
template: templates/one.yaml
lifecycle:
  create: Create first
  ready: Ready first
  cleanup: Cleanup first
---
name: fixture-two
description: Second fixture
template: templates/two.yaml
parameters:
  Param1: val1
lifecycle:
  create: Create second
  ready: Ready second
  cleanup: Cleanup second
---
name: fixture-three
description: Third fixture
templates:
  - templates/three-a.yaml
  - templates/three-b.yaml
lifecycle:
  create: Create third
  ready: Ready third
  cleanup: Cleanup third
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 3 {
		t.Errorf("expected 3 definitions, got %d", len(defs))
	}

	// Verify first fixture
	if defs[0].Name != "fixture-one" {
		t.Errorf("expected first fixture name 'fixture-one', got '%s'", defs[0].Name)
	}
	if defs[0].Description != "First fixture" {
		t.Errorf("expected first fixture description 'First fixture', got '%s'", defs[0].Description)
	}

	// Verify second fixture
	if defs[1].Name != "fixture-two" {
		t.Errorf("expected second fixture name 'fixture-two', got '%s'", defs[1].Name)
	}
	if defs[1].Description != "Second fixture" {
		t.Errorf("expected second fixture description 'Second fixture', got '%s'", defs[1].Description)
	}
	if len(defs[1].Parameters) != 1 {
		t.Errorf("expected 1 parameter in second fixture, got %d", len(defs[1].Parameters))
	}

	// Verify third fixture
	if defs[2].Name != "fixture-three" {
		t.Errorf("expected third fixture name 'fixture-three', got '%s'", defs[2].Name)
	}
	if len(defs[2].Templates) != 2 {
		t.Errorf("expected 2 templates in third fixture, got %d", len(defs[2].Templates))
	}
}

func TestLoadFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.yaml")

	defs, err := LoadFile(filePath)
	if err == nil {
		t.Fatal("expected LoadFile to return error for nonexistent file")
	}

	if defs != nil {
		t.Errorf("expected defs to be nil on error, got %v", defs)
	}

	// Verify error mentions the file path
	errMsg := err.Error()
	if !contains(errMsg, filePath) {
		t.Errorf("expected error message to contain file path %s, got: %s", filePath, errMsg)
	}
}

func TestLoadFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.yaml")

	err := os.WriteFile(filePath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 0 {
		t.Errorf("expected 0 definitions for empty file, got %d", len(defs))
	}
}

func TestLoadFile_OnlyEmptyDocuments(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty-docs.yaml")

	fixtureContent := `---
---
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 0 {
		t.Errorf("expected 0 definitions for empty documents, got %d", len(defs))
	}
}

func TestLoadFile_WithoutParameters(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "no-params.yaml")

	fixtureContent := `name: no-params-fixture
description: A fixture without parameters
template: templates/simple.yaml
lifecycle:
  create: Create it
  ready: Check it
  cleanup: Remove it
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("expected 1 definition, got %d", len(defs))
	}

	def := defs[0]
	if def.Parameters != nil && len(def.Parameters) != 0 {
		t.Errorf("expected empty parameters, got %v", def.Parameters)
	}
}

func TestLoadFile_TemplatesArray(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "multi-templates.yaml")

	fixtureContent := `name: multi-template-fixture
description: A fixture with multiple templates
templates:
  - templates/template1.yaml
  - templates/template2.yaml
  - templates/template3.yaml
lifecycle:
  create: Create all templates
  ready: All templates ready
  cleanup: Cleanup all templates
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("expected 1 definition, got %d", len(defs))
	}

	def := defs[0]
	if len(def.Templates) != 3 {
		t.Errorf("expected 3 templates, got %d", len(def.Templates))
	}

	if def.Templates[0] != "templates/template1.yaml" {
		t.Errorf("expected first template 'templates/template1.yaml', got '%s'", def.Templates[0])
	}

	if def.Templates[1] != "templates/template2.yaml" {
		t.Errorf("expected second template 'templates/template2.yaml', got '%s'", def.Templates[1])
	}

	if def.Templates[2] != "templates/template3.yaml" {
		t.Errorf("expected third template 'templates/template3.yaml', got '%s'", def.Templates[2])
	}
}

func TestLoadFile_MultipleParametersTypes(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "params.yaml")

	fixtureContent := `name: param-fixture
description: Test different parameter values
template: templates/param-test.yaml
parameters:
  StringParam: string-value
  NumberParam: "123"
  BoolParam: "true"
  EmptyParam: ""
  PathParam: /some/path/to/resource
lifecycle:
  create: Create
  ready: Ready
  cleanup: Cleanup
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	def := defs[0]
	if len(def.Parameters) != 5 {
		t.Errorf("expected 5 parameters, got %d", len(def.Parameters))
	}

	if def.Parameters["StringParam"] != "string-value" {
		t.Errorf("expected StringParam='string-value', got '%s'", def.Parameters["StringParam"])
	}

	if def.Parameters["NumberParam"] != "123" {
		t.Errorf("expected NumberParam='123', got '%s'", def.Parameters["NumberParam"])
	}

	if def.Parameters["BoolParam"] != "true" {
		t.Errorf("expected BoolParam='true', got '%s'", def.Parameters["BoolParam"])
	}

	if def.Parameters["EmptyParam"] != "" {
		t.Errorf("expected EmptyParam='', got '%s'", def.Parameters["EmptyParam"])
	}

	if def.Parameters["PathParam"] != "/some/path/to/resource" {
		t.Errorf("expected PathParam='/some/path/to/resource', got '%s'", def.Parameters["PathParam"])
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.yaml")

	invalidContent := `name: test
  invalid indentation:
    bad: yaml: syntax:
`

	err := os.WriteFile(filePath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err == nil {
		t.Fatal("expected LoadFile to return error for invalid YAML")
	}

	if defs != nil {
		t.Errorf("expected defs to be nil on error, got %v", defs)
	}

	errMsg := err.Error()
	if !contains(errMsg, "parsing YAML") {
		t.Errorf("expected error to mention parsing YAML, got: %s", errMsg)
	}
}

func TestLoadFile_MixedValidAndEmptyDocuments(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mixed.yaml")

	fixtureContent := `name: first-fixture
description: First one
template: templates/first.yaml
lifecycle:
  create: Create
  ready: Ready
  cleanup: Cleanup
---
---
name: second-fixture
description: Second one
template: templates/second.yaml
lifecycle:
  create: Create
  ready: Ready
  cleanup: Cleanup
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	// Should only have 2 definitions (empty documents are skipped)
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}

	if defs[0].Name != "first-fixture" {
		t.Errorf("expected first fixture name 'first-fixture', got '%s'", defs[0].Name)
	}

	if defs[1].Name != "second-fixture" {
		t.Errorf("expected second fixture name 'second-fixture', got '%s'", defs[1].Name)
	}
}

func TestLoadFile_WithWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "whitespace.yaml")

	fixtureContent := `
name: fixture-with-whitespace
description: Test whitespace handling
template: templates/test.yaml
parameters:
  Key1: value1
lifecycle:
  create: Create fixture
  ready: Ready check
  cleanup: Cleanup
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("expected 1 definition, got %d", len(defs))
	}

	if defs[0].Name != "fixture-with-whitespace" {
		t.Errorf("expected name 'fixture-with-whitespace', got '%s'", defs[0].Name)
	}
}

func TestLoadFile_FullExample(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "example.yaml")

	fixtureContent := `name: test-fixture
description: A test fixture
template: templates/test.yaml
parameters:
  Key1: value1
  Key2: value2
lifecycle:
  create: Apply the template
  ready: Wait for pod to be ready
  cleanup: Delete the pod
`

	err := os.WriteFile(filePath, []byte(fixtureContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	defs, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}

	def := defs[0]

	// Test all fields
	if def.Name != "test-fixture" {
		t.Errorf("Name mismatch")
	}
	if def.Description != "A test fixture" {
		t.Errorf("Description mismatch")
	}
	if def.Template != "templates/test.yaml" {
		t.Errorf("Template mismatch")
	}
	if len(def.Parameters) != 2 {
		t.Errorf("Parameters count mismatch")
	}
	if def.Lifecycle.Create != "Apply the template" {
		t.Errorf("Lifecycle.Create mismatch")
	}
	if def.Lifecycle.Ready != "Wait for pod to be ready" {
		t.Errorf("Lifecycle.Ready mismatch")
	}
	if def.Lifecycle.Cleanup != "Delete the pod" {
		t.Errorf("Lifecycle.Cleanup mismatch")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
