package fixture

// Definition represents a reusable fixture loaded from a YAML file.
// Fixtures define parameterized resources that specs can reference by name.
type Definition struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Status      string            `yaml:"status,omitempty"` // "draft" or "approved" (empty = approved)
	Template    string            `yaml:"template,omitempty"`    // Single template path
	Templates   []string          `yaml:"templates,omitempty"`  // Multiple template paths
	Parameters  map[string]string `yaml:"parameters,omitempty"` // Template parameters
	Lifecycle   Lifecycle         `yaml:"lifecycle"`
}

// EffectiveStatus returns the fixture status, defaulting to "approved" if empty.
func (d *Definition) EffectiveStatus() string {
	if d.Status == "" {
		return "approved"
	}
	return d.Status
}

// IsDraft returns true if the fixture is in draft status.
func (d *Definition) IsDraft() bool {
	return d.EffectiveStatus() == "draft"
}

// Lifecycle defines natural language descriptions of fixture lifecycle operations.
// The LLM agent interprets these to generate the actual tool calls.
type Lifecycle struct {
	Create  string `yaml:"create"`  // How to create the fixture
	Ready   string `yaml:"ready"`   // How to verify the fixture is ready
	Cleanup string `yaml:"cleanup"` // How to clean up the fixture
}

// TemplatePaths returns all template paths for this fixture.
func (d *Definition) TemplatePaths() []string {
	if d.Template != "" {
		return []string{d.Template}
	}
	return d.Templates
}

// Registry holds all loaded fixture definitions keyed by name.
type Registry struct {
	fixtures map[string]*Definition
}

// NewRegistry creates an empty fixture registry.
func NewRegistry() *Registry {
	return &Registry{
		fixtures: make(map[string]*Definition),
	}
}

// Register adds a fixture definition to the registry.
func (r *Registry) Register(def *Definition) {
	r.fixtures[def.Name] = def
}

// Get returns a fixture definition by name, or nil if not found.
func (r *Registry) Get(name string) *Definition {
	return r.fixtures[name]
}

// List returns all registered fixture names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.fixtures))
	for name := range r.fixtures {
		names = append(names, name)
	}
	return names
}

// Has returns true if a fixture with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.fixtures[name]
	return ok
}
