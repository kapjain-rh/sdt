package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if store == nil {
		t.Fatal("expected store to not be nil")
	}

	if store.cacheDir != tmpDir {
		t.Errorf("expected cacheDir %s, got %s", tmpDir, store.cacheDir)
	}

	// Verify directory structure was created
	expectedDirs := []string{
		tmpDir,
		filepath.Join(tmpDir, "plans"),
		filepath.Join(tmpDir, "templates"),
		filepath.Join(tmpDir, "results"),
	}

	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

func TestNewStore_CreatesDirectoriesWithCorrectPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Check that directories have expected permissions
	plansDir := filepath.Join(store.cacheDir, "plans")
	info, err := os.Stat(plansDir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	mode := info.Mode()
	if (mode & 0755) == 0 {
		t.Errorf("expected directory permissions to be set, got %o", mode)
	}
}

func TestComputeHash(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	content := []byte("test content")
	hash := store.ComputeHash(content)

	// Test that hash is a hex string
	if len(hash) != 64 { // SHA256 hex is 64 characters
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Verify it's valid hex by checking characters
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected hash to be hex string, got invalid character: %c", c)
		}
	}
}

func TestComputeHash_Consistency(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	content := []byte("test content")
	hash1 := store.ComputeHash(content)
	hash2 := store.ComputeHash(content)

	if hash1 != hash2 {
		t.Errorf("expected consistent hashes, got %s and %s", hash1, hash2)
	}
}

func TestComputeHash_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash1 := store.ComputeHash([]byte("content1"))
	hash2 := store.ComputeHash([]byte("content2"))

	if hash1 == hash2 {
		t.Errorf("expected different hashes for different content, got same hash")
	}
}

func TestSavePlan_GetPlan(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	planBytes := []byte(`{"spec_hash":"testhash123","spec_name":"test-spec","phases":[{"name":"setup","steps":[{"description":"Create test resource","tool_name":"kubectl"}]}]}`)

	hash := store.ComputeHash(planBytes)

	err = store.SavePlan(hash, planBytes)
	if err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	retrieved, ok := store.GetPlan(hash)
	if !ok {
		t.Fatal("expected GetPlan to return true")
	}

	if string(retrieved) != string(planBytes) {
		t.Errorf("expected retrieved plan to match saved plan")
	}
}

func TestSavePlan_FileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash := "abc123def456"
	planBytes := []byte(`{"spec_hash":"abc123def456","spec_name":"test"}`)

	err = store.SavePlan(hash, planBytes)
	if err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	// Verify file was created at correct path
	expectedPath := filepath.Join(tmpDir, "plans", hash+".json")
	_, err = os.Stat(expectedPath)
	if err != nil {
		t.Errorf("expected plan file at %s to exist: %v", expectedPath, err)
	}

	// Verify file content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read plan file: %v", err)
	}

	if string(content) != string(planBytes) {
		t.Errorf("expected file content to match saved content")
	}
}

func TestGetPlan_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	retrieved, ok := store.GetPlan("nonexistenthash")
	if ok {
		t.Fatal("expected GetPlan to return false for nonexistent hash")
	}

	if retrieved != nil {
		t.Fatal("expected retrieved content to be nil for nonexistent hash")
	}
}

func TestInvalidatePlan(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash := "test-hash-123"
	planBytes := []byte(`{"spec_hash":"test-hash-123"}`)

	// Save a plan
	err = store.SavePlan(hash, planBytes)
	if err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	// Verify it was saved
	_, ok := store.GetPlan(hash)
	if !ok {
		t.Fatal("expected plan to be saved")
	}

	// Invalidate the plan
	err = store.InvalidatePlan(hash)
	if err != nil {
		t.Fatalf("InvalidatePlan failed: %v", err)
	}

	// Verify it was removed
	_, ok = store.GetPlan(hash)
	if ok {
		t.Fatal("expected plan to be removed after invalidation")
	}
}

func TestInvalidatePlan_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Should not error when invalidating nonexistent plan
	err = store.InvalidatePlan("nonexistenthash")
	if err != nil {
		t.Errorf("expected no error when invalidating nonexistent plan, got %v", err)
	}
}

func TestInvalidatePlan_FileRemoved(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash := "test-hash-456"
	planBytes := []byte(`{"spec_hash":"test-hash-456"}`)

	// Save a plan
	err = store.SavePlan(hash, planBytes)
	if err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "plans", hash+".json")

	// Verify file exists
	_, err = os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("expected plan file to exist before invalidation")
	}

	// Invalidate
	err = store.InvalidatePlan(hash)
	if err != nil {
		t.Fatalf("InvalidatePlan failed: %v", err)
	}

	// Verify file is gone
	_, err = os.Stat(expectedPath)
	if err == nil {
		t.Fatal("expected plan file to be deleted after invalidation")
	}
	if !os.IsNotExist(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMarshalJSON(t *testing.T) {
	input := map[string]string{"spec_hash": "test-hash", "spec_name": "test-spec"}

	data, err := MarshalJSON(input)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected marshaled data to not be empty")
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	data := []byte(`{"spec_hash":"test-hash","spec_name":"test-spec"}`)

	var result map[string]string
	err := UnmarshalJSON(data, &result)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if result["spec_hash"] != "test-hash" {
		t.Errorf("expected spec_hash %q, got %q", "test-hash", result["spec_hash"])
	}
	if result["spec_name"] != "test-spec" {
		t.Errorf("expected spec_name %q, got %q", "test-spec", result["spec_name"])
	}
}

func TestSavePlan_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash := "overwrite-test"
	plan1 := []byte(`{"spec_name":"plan1"}`)
	plan2 := []byte(`{"spec_name":"plan2"}`)

	// Save first plan
	err = store.SavePlan(hash, plan1)
	if err != nil {
		t.Fatalf("first SavePlan failed: %v", err)
	}

	// Save second plan with same hash
	err = store.SavePlan(hash, plan2)
	if err != nil {
		t.Fatalf("second SavePlan failed: %v", err)
	}

	// Retrieve and verify it's the second plan
	retrieved, ok := store.GetPlan(hash)
	if !ok {
		t.Fatal("expected plan to be retrieved")
	}

	if string(retrieved) != string(plan2) {
		t.Errorf("expected retrieved plan to be plan2")
	}
}

func TestComputeHash_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash := store.ComputeHash([]byte(""))

	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Verify it's consistent
	hash2 := store.ComputeHash([]byte(""))
	if hash != hash2 {
		t.Errorf("expected consistent hash for empty content")
	}
}

func TestMultipleStores(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	store1, err := NewStore(tmpDir1)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	store2, err := NewStore(tmpDir2)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	hash := "shared-hash"
	plan1 := []byte(`{"version":"1"}`)
	plan2 := []byte(`{"version":"2"}`)

	// Save different plans to different stores
	err = store1.SavePlan(hash, plan1)
	if err != nil {
		t.Fatalf("SavePlan to store1 failed: %v", err)
	}

	err = store2.SavePlan(hash, plan2)
	if err != nil {
		t.Fatalf("SavePlan to store2 failed: %v", err)
	}

	// Verify they're independent
	retrieved1, ok := store1.GetPlan(hash)
	if !ok {
		t.Fatal("expected plan from store1")
	}

	retrieved2, ok := store2.GetPlan(hash)
	if !ok {
		t.Fatal("expected plan from store2")
	}

	if string(retrieved1) != string(plan1) {
		t.Errorf("expected store1 to have plan1")
	}

	if string(retrieved2) != string(plan2) {
		t.Errorf("expected store2 to have plan2")
	}
}
