package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TemplateRegistry holds all registered template definitions.
type TemplateRegistry struct {
	mu        sync.RWMutex
	templates map[string]*TemplateDef
	baseDir   string
}

// NewTemplateRegistry creates a new template registry with a base directory.
func NewTemplateRegistry(baseDir string) *TemplateRegistry {
	return &TemplateRegistry{
		templates: make(map[string]*TemplateDef),
		baseDir:   baseDir,
	}
}

// Register adds a template definition to the registry.
func (r *TemplateRegistry) Register(def *TemplateDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[def.Name] = def
}

// Get returns a template definition by name, or nil if not found.
func (r *TemplateRegistry) Get(name string) *TemplateDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.templates[name]
}

// List returns all registered template names.
func (r *TemplateRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}

// Has returns true if a template with the given name is registered.
func (r *TemplateRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.templates[name]
	return ok
}

// LoadDir scans a templates directory and auto-registers templates by relative path.
// Each .yaml or .yml file in the directory tree becomes a template named by its relative path.
func (r *TemplateRegistry) LoadDir(dir string) error {
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Compute relative path as template name
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Remove extension for the template name
		name := strings.TrimSuffix(relPath, ext)
		// Normalize path separators to forward slashes for consistency
		name = strings.ReplaceAll(name, "\\", "/")

		def := &TemplateDef{
			Name:        name,
			Path:        path,
			Description: fmt.Sprintf("Template loaded from %s", relPath),
		}
		r.Register(def)

		return nil
	}); err != nil {
		return err
	}

	return nil
}
