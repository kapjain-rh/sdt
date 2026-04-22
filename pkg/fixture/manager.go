package fixture

import (
	"fmt"
)

// Manager handles fixture lifecycle operations.
// It resolves fixture names to definitions and provides the lifecycle
// descriptions that the LLM agent uses to create/verify/cleanup fixtures.
type Manager struct {
	registry *Registry
	baseDir  string // Base directory for resolving relative template paths
}

// NewManager creates a fixture manager with the given registry and base directory.
func NewManager(registry *Registry, baseDir string) *Manager {
	return &Manager{
		registry: registry,
		baseDir:  baseDir,
	}
}

// Resolve looks up fixture definitions for a list of fixture names.
// Returns an error if any fixture name is not found in the registry.
func (m *Manager) Resolve(names []string) ([]*Definition, error) {
	var defs []*Definition
	for _, name := range names {
		def := m.registry.Get(name)
		if def == nil {
			return nil, fmt.Errorf("fixture %q not found in registry", name)
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// CreateContext builds the context string for the LLM agent describing
// what fixtures to create, their templates, parameters, and lifecycle.
func (m *Manager) CreateContext(defs []*Definition) string {
	if len(defs) == 0 {
		return ""
	}

	ctx := "Fixtures to set up:\n"
	for _, def := range defs {
		ctx += fmt.Sprintf("\n### Fixture: %s\n", def.Name)
		ctx += fmt.Sprintf("Description: %s\n", def.Description)

		templates := def.TemplatePaths()
		if len(templates) > 0 {
			ctx += "Templates:\n"
			for _, t := range templates {
				ctx += fmt.Sprintf("  - %s\n", t)
			}
		}

		if len(def.Parameters) > 0 {
			ctx += "Parameters:\n"
			for k, v := range def.Parameters {
				ctx += fmt.Sprintf("  %s: %s\n", k, v)
			}
		}

		ctx += fmt.Sprintf("Create: %s\n", def.Lifecycle.Create)
		ctx += fmt.Sprintf("Ready check: %s\n", def.Lifecycle.Ready)
	}

	return ctx
}

// CleanupContext builds the context string for fixture cleanup.
// Fixtures are listed in reverse order for proper teardown.
func (m *Manager) CleanupContext(defs []*Definition) string {
	if len(defs) == 0 {
		return ""
	}

	ctx := "Fixtures to clean up (in reverse order):\n"
	for i := len(defs) - 1; i >= 0; i-- {
		def := defs[i]
		ctx += fmt.Sprintf("\n### Fixture: %s\n", def.Name)
		ctx += fmt.Sprintf("Cleanup: %s\n", def.Lifecycle.Cleanup)
	}

	return ctx
}

// Registry returns the underlying fixture registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}
