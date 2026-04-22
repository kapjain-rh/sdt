package template

// TemplateDef represents a reusable template definition.
type TemplateDef struct {
	Name        string           `yaml:"name"`
	Path        string           `yaml:"path"`
	Description string           `yaml:"description"`
	Parameters  []TemplateParam  `yaml:"parameters,omitempty"`
	Generated   bool             `yaml:"generated,omitempty"`
	Hash        string           `yaml:"hash,omitempty"`
}

// TemplateParam represents a single template parameter.
type TemplateParam struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}
