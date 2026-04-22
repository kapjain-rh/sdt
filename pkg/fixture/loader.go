package fixture

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadDir loads all fixture definitions from YAML files in a directory.
// Each YAML file can contain multiple fixture definitions separated by "---".
func LoadDir(dir string) (*Registry, error) {
	registry := NewRegistry()

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil // No fixtures directory is fine
		}
		return nil, fmt.Errorf("accessing fixtures directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading fixtures directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		defs, err := LoadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("loading fixture file %s: %w", name, err)
		}

		for _, def := range defs {
			registry.Register(def)
		}
	}

	return registry, nil
}

// LoadFile loads fixture definitions from a single YAML file.
// Supports multi-document YAML (separated by "---").
func LoadFile(path string) ([]*Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var defs []*Definition
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var def Definition
		err := decoder.Decode(&def)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing YAML in %s: %w", path, err)
		}
		if def.Name == "" {
			continue // Skip empty documents
		}
		defs = append(defs, &def)
	}

	return defs, nil
}

// LoadDirRecursive loads fixtures from a directory tree.
func LoadDirRecursive(root string) (*Registry, error) {
	registry := NewRegistry()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			return nil
		}

		defs, err := LoadFile(path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", path, err)
		}
		for _, def := range defs {
			registry.Register(def)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return registry, nil
}
