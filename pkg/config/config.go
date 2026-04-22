package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the contents of an .sdt.yaml configuration file.
type Config struct {
	Project     string                       `yaml:"project"`
	Description string                       `yaml:"description,omitempty"`
	Context     string                       `yaml:"context,omitempty"`
	SpecsDir    string                       `yaml:"specsDir,omitempty"`
	FixturesDir string                       `yaml:"fixturesDir,omitempty"`
	ToolsDir    string                       `yaml:"toolsDir,omitempty"`
	MCPServers  map[string]MCPServerConfig   `yaml:"mcpServers,omitempty"`
	Extra       map[string]interface{}       `yaml:"extra,omitempty"`
}

// MCPServerConfig describes how to launch an MCP server process.
type MCPServerConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

var defaultConfigFiles = []string{".sdt.yaml", ".sdt.yml"}

// Load searches for a config file in the given directory,
// parses it, and returns the Config. Returns a zero-value Config
// if no config file is found (config is optional).
func Load(dir string) (*Config, error) {
	for _, name := range defaultConfigFiles {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		return &cfg, nil
	}
	return &Config{}, nil
}

// LoadFile loads config from a specific file path.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}
