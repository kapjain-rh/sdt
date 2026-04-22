package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sdt-project/sdt/pkg/cache"
	"github.com/sdt-project/sdt/pkg/spec"
)

// MemoryAgent manages caching of plans and results for test specs.
type MemoryAgent struct {
	store *cache.Store
}

// NewMemoryAgent creates a new memory agent.
func NewMemoryAgent(store *cache.Store) *MemoryAgent {
	return &MemoryAgent{
		store: store,
	}
}

// GetCachedPlan retrieves a cached plan for a spec if it exists.
func (m *MemoryAgent) GetCachedPlan(spec *spec.TestSpec) (*ExecutionPlan, bool) {
	specHash := m.ComputeSpecHash(spec)
	cached, ok := m.store.GetPlan(specHash)
	if !ok {
		return nil, false
	}

	var plan ExecutionPlan
	if err := json.Unmarshal(cached, &plan); err != nil {
		return nil, false
	}

	return &plan, true
}

// SavePlan saves an execution plan to memory/cache.
func (m *MemoryAgent) SavePlan(spec *spec.TestSpec, plan *ExecutionPlan) error {
	specHash := m.ComputeSpecHash(spec)
	planBytes, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshaling plan: %w", err)
	}
	return m.store.SavePlan(specHash, planBytes)
}

// SaveResult saves an execution result to history.
func (m *MemoryAgent) SaveResult(spec *spec.TestSpec, result *ExecutionResult) error {
	specHash := m.ComputeSpecHash(spec)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	runID := fmt.Sprintf("%d", result.StartTime.Unix())
	hist := cache.NewHistory(m.store)
	return hist.SaveResult(specHash, runID, resultBytes)
}

// InvalidateCache removes cached plan and results for a spec.
func (m *MemoryAgent) InvalidateCache(spec *spec.TestSpec) error {
	specHash := m.ComputeSpecHash(spec)
	if err := m.store.InvalidatePlan(specHash); err != nil {
		return err
	}
	return nil
}

// ComputeSpecHash computes a hash of the spec file content for caching.
// This ensures that if the spec file is modified, the cache is invalidated.
func (m *MemoryAgent) ComputeSpecHash(spec *spec.TestSpec) string {
	// Try to read the spec file and hash its content
	if content, err := os.ReadFile(spec.FilePath); err == nil {
		hash := sha256.Sum256(content)
		return hex.EncodeToString(hash[:])
	}

	// Fallback: hash the spec struct fields
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d|%d|%d",
		spec.Name,
		spec.FilePath,
		spec.Metadata.Author,
		spec.Metadata.Priority,
		spec.Metadata.CaseID,
		len(spec.Setup),
		len(spec.Steps),
		len(spec.Verify),
		len(spec.Cleanup),
	)

	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

// GetLatestResult retrieves the most recent result for a spec.
func (m *MemoryAgent) GetLatestResult(spec *spec.TestSpec) (*ExecutionResult, error) {
	specHash := m.ComputeSpecHash(spec)
	hist := cache.NewHistory(m.store)
	resultBytes, err := hist.GetLatestResult(specHash)
	if err != nil {
		return nil, err
	}

	var result ExecutionResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling result: %w", err)
	}

	return &result, nil
}

// GetHistory retrieves all past results for a spec.
func (m *MemoryAgent) GetHistory(spec *spec.TestSpec) ([]*ExecutionResult, error) {
	specHash := m.ComputeSpecHash(spec)
	hist := cache.NewHistory(m.store)
	resultBytes, err := hist.GetResults(specHash)
	if err != nil {
		return nil, err
	}

	var results []*ExecutionResult
	for _, rb := range resultBytes {
		var result ExecutionResult
		if err := json.Unmarshal(rb, &result); err != nil {
			continue
		}
		results = append(results, &result)
	}

	return results, nil
}
