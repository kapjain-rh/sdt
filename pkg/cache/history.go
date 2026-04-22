package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// History manages storing and retrieving test execution results over time.
type History struct {
	store *Store
}

// NewHistory creates a new history manager.
func NewHistory(store *Store) *History {
	return &History{
		store: store,
	}
}

// ResultRecord represents a single stored result with metadata.
type ResultRecord struct {
	SpecHash   string    `json:"spec_hash"`
	RunID      string    `json:"run_id"`
	Timestamp  time.Time `json:"timestamp"`
	Content    []byte    `json:"-"` // Not marshaled, stored separately
	Checksum   string    `json:"checksum"`
}

// SaveResult saves an execution result to history with a run ID.
// The content is expected to be JSON (e.g., serialized ExecutionResult).
func (h *History) SaveResult(specHash, runID string, result []byte) error {
	// Create a results directory for this spec hash
	resultDir := filepath.Join(h.store.cacheDir, "results", specHash)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return fmt.Errorf("creating results directory: %w", err)
	}

	// Save the result with timestamp-based naming for uniqueness
	now := time.Now()
	filename := fmt.Sprintf("%s-%d.json", runID, now.UnixNano())
	resultPath := filepath.Join(resultDir, filename)

	if err := os.WriteFile(resultPath, result, 0644); err != nil {
		return fmt.Errorf("writing result: %w", err)
	}

	// Save metadata record
	record := ResultRecord{
		SpecHash:  specHash,
		RunID:     runID,
		Timestamp: now,
		Checksum:  h.store.ComputeHash(result),
	}

	metaData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	metaPath := filepath.Join(resultDir, filename+".meta")
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}

// GetResults retrieves all past results for a given spec hash, sorted by timestamp (newest first).
func (h *History) GetResults(specHash string) ([][]byte, error) {
	resultDir := filepath.Join(h.store.cacheDir, "results", specHash)

	// Check if directory exists
	if _, err := os.Stat(resultDir); os.IsNotExist(err) {
		return [][]byte{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("accessing results directory: %w", err)
	}

	// List all JSON files in the directory
	entries, err := os.ReadDir(resultDir)
	if err != nil {
		return nil, fmt.Errorf("reading results directory: %w", err)
	}

	var results [][]byte
	var records []ResultRecord

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip metadata files
		if filepath.Ext(entry.Name()) == ".meta" {
			continue
		}

		// Read the result file
		path := filepath.Join(resultDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Try to load associated metadata for sorting
		metaPath := path + ".meta"
		if metaContent, err := os.ReadFile(metaPath); err == nil {
			var record ResultRecord
			if err := json.Unmarshal(metaContent, &record); err == nil {
				record.Content = content
				records = append(records, record)
				continue
			}
		}

		// If no metadata, just add the content
		results = append(results, content)
	}

	// Sort by timestamp (newest first)
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	// Combine sorted records with unsorted results
	sortedResults := make([][]byte, 0, len(records)+len(results))
	for _, record := range records {
		sortedResults = append(sortedResults, record.Content)
	}
	sortedResults = append(sortedResults, results...)

	return sortedResults, nil
}

// GetLatestResult retrieves the most recent result for a given spec hash.
func (h *History) GetLatestResult(specHash string) ([]byte, error) {
	results, err := h.GetResults(specHash)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for spec hash %s", specHash)
	}

	return results[0], nil
}

// ClearResultsForSpec removes all results for a given spec hash.
func (h *History) ClearResultsForSpec(specHash string) error {
	resultDir := filepath.Join(h.store.cacheDir, "results", specHash)
	if err := os.RemoveAll(resultDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clearing results: %w", err)
	}
	return nil
}

// GetResultCount returns the number of stored results for a spec hash.
func (h *History) GetResultCount(specHash string) (int, error) {
	results, err := h.GetResults(specHash)
	if err != nil {
		return 0, err
	}
	return len(results), nil
}
