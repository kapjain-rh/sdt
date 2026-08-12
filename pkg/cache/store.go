package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store manages caching of plans and generated content.
type Store struct {
	cacheDir string
}

// NewStore creates a new cache store and initializes the cache directory structure.
func NewStore(cacheDir string) (*Store, error) {
	// Create cache directories if they don't exist
	dirs := []string{
		cacheDir,
		filepath.Join(cacheDir, "plans"),
		filepath.Join(cacheDir, "templates"),
		filepath.Join(cacheDir, "results"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating cache directory %s: %w", dir, err)
		}
	}

	return &Store{
		cacheDir: cacheDir,
	}, nil
}

// GetPlan retrieves a cached execution plan by spec hash.
// Returns the plan bytes and a boolean indicating whether it was found.
func (s *Store) GetPlan(specHash string) ([]byte, bool) {
	path := filepath.Join(s.cacheDir, "plans", specHash+".json")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return content, true
}

// SavePlan stores an execution plan to cache.
func (s *Store) SavePlan(specHash string, plan []byte) error {
	path := filepath.Join(s.cacheDir, "plans", specHash+".json")
	if err := os.WriteFile(path, plan, 0644); err != nil {
		return fmt.Errorf("writing plan cache: %w", err)
	}
	return nil
}

// GetGeneratedTemplate retrieves a cached generated template by description hash.
// Returns the template content bytes and a boolean indicating whether it was found.
func (s *Store) GetGeneratedTemplate(descHash string) ([]byte, bool) {
	path := filepath.Join(s.cacheDir, "templates", descHash+".yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return content, true
}

// SaveGeneratedTemplate stores a generated template and its metadata to cache.
// The content is the template YAML, and meta is optional metadata (as JSON).
func (s *Store) SaveGeneratedTemplate(descHash string, content []byte, meta []byte) error {
	// Save the template content
	templatePath := filepath.Join(s.cacheDir, "templates", descHash+".yaml")
	if err := os.WriteFile(templatePath, content, 0644); err != nil {
		return fmt.Errorf("writing template cache: %w", err)
	}

	// Save metadata if provided
	if len(meta) > 0 {
		metaPath := filepath.Join(s.cacheDir, "templates", descHash+".meta.json")
		if err := os.WriteFile(metaPath, meta, 0644); err != nil {
			return fmt.Errorf("writing template metadata: %w", err)
		}
	}

	return nil
}

// ComputeHash computes a SHA256 hash of the given content and returns it as a hex string.
func (s *Store) ComputeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// ComputeContentHash computes a SHA256 hash of a string and returns it as a hex string.
func ComputeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// InvalidatePlan removes a cached plan by spec hash.
func (s *Store) InvalidatePlan(specHash string) error {
	path := filepath.Join(s.cacheDir, "plans", specHash+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plan cache: %w", err)
	}
	return nil
}

// InvalidateTemplate removes cached template files by description hash.
func (s *Store) InvalidateTemplate(descHash string) error {
	templatePath := filepath.Join(s.cacheDir, "templates", descHash+".yaml")
	metaPath := filepath.Join(s.cacheDir, "templates", descHash+".meta.json")

	if err := os.Remove(templatePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing template cache: %w", err)
	}

	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing template metadata: %w", err)
	}

	return nil
}

// GetCacheSize returns the total size of cached files in bytes.
func (s *Store) GetCacheSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(s.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}

// ClearCache removes all cached files.
func (s *Store) ClearCache() error {
	return os.RemoveAll(s.cacheDir)
}

// ComputeSpecHashFromFile computes a SHA256 hash of the spec file content.
// If the file cannot be read, it falls back to hashing the provided fallback fields.
func ComputeSpecHashFromFile(filePath string, fallbackFields ...string) string {
	if content, err := os.ReadFile(filePath); err == nil {
		hash := sha256.Sum256(content)
		return hex.EncodeToString(hash[:])
	}
	combined := strings.Join(fallbackFields, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// ComputeHookHash computes a SHA256 hash of hook phase name and step texts.
func ComputeHookHash(phaseName string, stepTexts []string) string {
	combined := phaseName
	for _, s := range stepTexts {
		combined += "|" + s
	}
	hash := sha256.Sum256([]byte(combined))
	return "hook-" + hex.EncodeToString(hash[:])
}

// MarshalJSON marshals an object to JSON bytes for caching.
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON unmarshals JSON bytes into an object.
func UnmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
