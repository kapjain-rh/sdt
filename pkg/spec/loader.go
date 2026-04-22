package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadSuite loads a complete suite from a directory: _suite.md, _group_*.md, and all test specs.
func LoadSuite(dir string) (*Suite, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("accessing directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	suite := &Suite{
		Dir:    dir,
		Groups: make(map[string]*GroupSpec),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		name := entry.Name()

		switch {
		case name == "_suite.md":
			s, err := ParseSuiteSpec(fullPath)
			if err != nil {
				return nil, fmt.Errorf("parsing suite spec: %w", err)
			}
			suite.SuiteSpec = s

		case strings.HasPrefix(name, "_group_") && strings.HasSuffix(name, ".md"):
			g, err := ParseGroupSpec(fullPath)
			if err != nil {
				return nil, fmt.Errorf("parsing group spec %s: %w", name, err)
			}
			// Derive group name from filename if not set in the file
			if g.Name == "" {
				g.Name = strings.TrimSuffix(strings.TrimPrefix(name, "_group_"), ".md")
			}
			suite.Groups[g.Name] = g

		case !strings.HasPrefix(name, "_"):
			t, err := ParseTestSpec(fullPath)
			if err != nil {
				return nil, fmt.Errorf("parsing test spec %s: %w", name, err)
			}
			suite.Tests = append(suite.Tests, t)
		}
	}

	return suite, nil
}

// LoadTestSpec loads a single test spec from a file path.
// If the path is a directory, it loads all test specs in it (without suite/group).
func LoadTestSpec(path string) ([]*TestSpec, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("accessing %s: %w", path, err)
	}

	if !info.IsDir() {
		spec, err := ParseTestSpec(path)
		if err != nil {
			return nil, err
		}
		return []*TestSpec{spec}, nil
	}

	// Load all .md files in directory (excluding _ prefixed)
	var specs []*TestSpec
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", path, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		spec, err := ParseTestSpec(filepath.Join(path, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// LoadSuiteRecursive loads suites from a directory tree.
// Each subdirectory with .md files is treated as a suite.
func LoadSuiteRecursive(root string) ([]*Suite, error) {
	var suites []*Suite

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		// Check if this directory has any .md files
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		hasMD := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !strings.HasPrefix(e.Name(), "_") {
				hasMD = true
				break
			}
		}
		if !hasMD {
			return nil
		}

		suite, err := LoadSuite(path)
		if err != nil {
			return fmt.Errorf("loading suite from %s: %w", path, err)
		}
		suites = append(suites, suite)
		return nil
	})

	return suites, err
}
