package spec

import (
	"os"
	"testing"
	"time"
)

func TestParseTestSpec_FullSpec(t *testing.T) {
	content := `# Test: Verify Something Works

## Metadata
- Author: tester
- Priority: High
- CaseID: 42
- Labels: [Serial, Slow]
- Timeout: 15m
- Group: my-group
- Fixtures: [fixture-a, fixture-b]

## Setup
1. Prepare the environment.
2. Configure the system.

## Steps
1. Do the first thing.
2. Do the second thing.

## Verify
- Check that result A is correct.
- Check that result B is correct.

## Cleanup
1. Remove all test resources.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if spec.Name != "Verify Something Works" {
		t.Errorf("expected Name 'Verify Something Works', got '%s'", spec.Name)
	}

	if spec.FilePath != tmpFile {
		t.Errorf("expected FilePath %s, got %s", tmpFile, spec.FilePath)
	}

	if spec.Metadata.Author != "tester" {
		t.Errorf("expected Author 'tester', got '%s'", spec.Metadata.Author)
	}

	if spec.Metadata.Priority != "High" {
		t.Errorf("expected Priority 'High', got '%s'", spec.Metadata.Priority)
	}

	if spec.Metadata.CaseID != "42" {
		t.Errorf("expected CaseID '42', got '%s'", spec.Metadata.CaseID)
	}

	if len(spec.Metadata.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(spec.Metadata.Labels))
	}
	if len(spec.Metadata.Labels) > 0 && spec.Metadata.Labels[0] != "Serial" {
		t.Errorf("expected first label 'Serial', got '%s'", spec.Metadata.Labels[0])
	}

	expectedTimeout := 15 * time.Minute
	if spec.Metadata.Timeout != expectedTimeout {
		t.Errorf("expected Timeout %v, got %v", expectedTimeout, spec.Metadata.Timeout)
	}

	if spec.Metadata.Group != "my-group" {
		t.Errorf("expected Group 'my-group', got '%s'", spec.Metadata.Group)
	}

	if len(spec.Metadata.Fixtures) != 2 {
		t.Errorf("expected 2 fixtures, got %d", len(spec.Metadata.Fixtures))
	}
	if len(spec.Metadata.Fixtures) > 0 && spec.Metadata.Fixtures[0] != "fixture-a" {
		t.Errorf("expected first fixture 'fixture-a', got '%s'", spec.Metadata.Fixtures[0])
	}

	if len(spec.Setup) != 2 {
		t.Errorf("expected 2 Setup steps, got %d", len(spec.Setup))
	}
	if len(spec.Setup) > 0 {
		if spec.Setup[0].RawText != "Prepare the environment." {
			t.Errorf("expected first Setup step 'Prepare the environment.', got '%s'", spec.Setup[0].RawText)
		}
		if spec.Setup[0].Phase != PhaseSetup {
			t.Errorf("expected Setup step Phase PhaseSetup, got %s", spec.Setup[0].Phase)
		}
		if spec.Setup[0].Index != 0 {
			t.Errorf("expected Setup step Index 0, got %d", spec.Setup[0].Index)
		}
	}

	if len(spec.Steps) != 2 {
		t.Errorf("expected 2 Steps, got %d", len(spec.Steps))
	}
	if len(spec.Steps) > 0 {
		if spec.Steps[0].RawText != "Do the first thing." {
			t.Errorf("expected first Step 'Do the first thing.', got '%s'", spec.Steps[0].RawText)
		}
		if spec.Steps[0].Phase != PhaseSteps {
			t.Errorf("expected Step Phase PhaseSteps, got %s", spec.Steps[0].Phase)
		}
		if spec.Steps[0].Index != 0 {
			t.Errorf("expected Step Index 0, got %d", spec.Steps[0].Index)
		}
	}

	if len(spec.Verify) != 2 {
		t.Errorf("expected 2 Verify steps, got %d", len(spec.Verify))
	}
	if len(spec.Verify) > 0 {
		if spec.Verify[0].RawText != "Check that result A is correct." {
			t.Errorf("expected first Verify step 'Check that result A is correct.', got '%s'", spec.Verify[0].RawText)
		}
		if spec.Verify[0].Phase != PhaseVerify {
			t.Errorf("expected Verify step Phase PhaseVerify, got %s", spec.Verify[0].Phase)
		}
		if spec.Verify[0].Index != 0 {
			t.Errorf("expected Verify step Index 0, got %d", spec.Verify[0].Index)
		}
	}

	if len(spec.Cleanup) != 1 {
		t.Errorf("expected 1 Cleanup step, got %d", len(spec.Cleanup))
	}
	if len(spec.Cleanup) > 0 {
		if spec.Cleanup[0].RawText != "Remove all test resources." {
			t.Errorf("expected Cleanup step 'Remove all test resources.', got '%s'", spec.Cleanup[0].RawText)
		}
		if spec.Cleanup[0].Phase != PhaseCleanup {
			t.Errorf("expected Cleanup step Phase PhaseCleanup, got %s", spec.Cleanup[0].Phase)
		}
	}
}

func TestParseTestSpec_MinimalSpec(t *testing.T) {
	content := `# Test: Minimal Test

## Steps
1. Do something.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if spec.Name != "Minimal Test" {
		t.Errorf("expected Name 'Minimal Test', got '%s'", spec.Name)
	}

	if len(spec.Steps) != 1 {
		t.Errorf("expected 1 Step, got %d", len(spec.Steps))
	}

	if len(spec.Setup) != 0 {
		t.Errorf("expected 0 Setup steps, got %d", len(spec.Setup))
	}

	if len(spec.Verify) != 0 {
		t.Errorf("expected 0 Verify steps, got %d", len(spec.Verify))
	}

	if len(spec.Cleanup) != 0 {
		t.Errorf("expected 0 Cleanup steps, got %d", len(spec.Cleanup))
	}
}

func TestParseTestSpec_BulletPoints(t *testing.T) {
	content := `# Test: Bullet Test

## Steps
- First step with bullet.
- Second step with bullet.
* Third step with asterisk.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if len(spec.Steps) != 3 {
		t.Errorf("expected 3 Steps, got %d", len(spec.Steps))
	}

	if spec.Steps[0].RawText != "First step with bullet." {
		t.Errorf("expected 'First step with bullet.', got '%s'", spec.Steps[0].RawText)
	}

	if spec.Steps[2].RawText != "Third step with asterisk." {
		t.Errorf("expected 'Third step with asterisk.', got '%s'", spec.Steps[2].RawText)
	}
}

func TestParseTestSpec_MetadataFields(t *testing.T) {
	content := `# Test: Metadata Test

## Metadata
- Author: john-doe
- Priority: Critical
- CaseID: TC-001
- Labels: [Quick, Destructive, Networking]
- Timeout: 30s
- Group: test-group
- Fixtures: [fixture1, fixture2, fixture3]

## Steps
1. Test step.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if spec.Metadata.Author != "john-doe" {
		t.Errorf("expected Author 'john-doe', got '%s'", spec.Metadata.Author)
	}

	if spec.Metadata.Priority != "Critical" {
		t.Errorf("expected Priority 'Critical', got '%s'", spec.Metadata.Priority)
	}

	if spec.Metadata.CaseID != "TC-001" {
		t.Errorf("expected CaseID 'TC-001', got '%s'", spec.Metadata.CaseID)
	}

	if len(spec.Metadata.Labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(spec.Metadata.Labels))
	}

	expectedTimeout := 30 * time.Second
	if spec.Metadata.Timeout != expectedTimeout {
		t.Errorf("expected Timeout %v, got %v", expectedTimeout, spec.Metadata.Timeout)
	}

	if spec.Metadata.Group != "test-group" {
		t.Errorf("expected Group 'test-group', got '%s'", spec.Metadata.Group)
	}

	if len(spec.Metadata.Fixtures) != 3 {
		t.Errorf("expected 3 fixtures, got %d", len(spec.Metadata.Fixtures))
	}
}

func TestParseTestSpec_TimeoutParsing(t *testing.T) {
	testCases := []struct {
		name            string
		timeoutValue    string
		expectedTimeout time.Duration
	}{
		{"minutes", "5m", 5 * time.Minute},
		{"seconds", "90s", 90 * time.Second},
		{"hours", "1h", 1 * time.Hour},
		{"combined", "1h30m", time.Hour + 30*time.Minute},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := `# Test: Timeout Test

## Metadata
- Timeout: ` + tc.timeoutValue + `

## Steps
1. Test.
`
			tmpFile := createTempFile(t, content)
			defer os.Remove(tmpFile)

			spec, err := ParseTestSpec(tmpFile)
			if err != nil {
				t.Fatalf("ParseTestSpec failed: %v", err)
			}

			if spec.Metadata.Timeout != tc.expectedTimeout {
				t.Errorf("expected Timeout %v, got %v", tc.expectedTimeout, spec.Metadata.Timeout)
			}
		})
	}
}

func TestParseTestSpec_NonExistentFile(t *testing.T) {
	_, err := ParseTestSpec("/nonexistent/file/path.md")
	if err == nil {
		t.Fatalf("expected error for non-existent file, got nil")
	}
}

func TestParseSuiteSpec_FullSpec(t *testing.T) {
	content := `# Suite: Full Suite

## Metadata
- Author: suite-author
- Priority: High

## Pre-Suite
1. Initialize global state.
2. Start services.

## Pre-Test
- Clear cache before each test.
- Reset database state.

## Post-Test
- Collect logs.

## Post-Suite
1. Shut down services.
2. Cleanup global resources.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	suite, err := ParseSuiteSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseSuiteSpec failed: %v", err)
	}

	if suite.Name != "Full Suite" {
		t.Errorf("expected Name 'Full Suite', got '%s'", suite.Name)
	}

	if suite.FilePath != tmpFile {
		t.Errorf("expected FilePath %s, got %s", tmpFile, suite.FilePath)
	}

	if suite.Metadata.Author != "suite-author" {
		t.Errorf("expected Author 'suite-author', got '%s'", suite.Metadata.Author)
	}

	if suite.Metadata.Priority != "High" {
		t.Errorf("expected Priority 'High', got '%s'", suite.Metadata.Priority)
	}

	if len(suite.PreSuite) != 2 {
		t.Errorf("expected 2 PreSuite steps, got %d", len(suite.PreSuite))
	}
	if len(suite.PreSuite) > 0 {
		if suite.PreSuite[0].RawText != "Initialize global state." {
			t.Errorf("expected 'Initialize global state.', got '%s'", suite.PreSuite[0].RawText)
		}
		if suite.PreSuite[0].Phase != PhasePreSuite {
			t.Errorf("expected Phase PhasePreSuite, got %s", suite.PreSuite[0].Phase)
		}
		if suite.PreSuite[0].Index != 0 {
			t.Errorf("expected Index 0, got %d", suite.PreSuite[0].Index)
		}
	}

	if len(suite.PreTest) != 2 {
		t.Errorf("expected 2 PreTest steps, got %d", len(suite.PreTest))
	}
	if len(suite.PreTest) > 0 {
		if suite.PreTest[0].RawText != "Clear cache before each test." {
			t.Errorf("expected 'Clear cache before each test.', got '%s'", suite.PreTest[0].RawText)
		}
		if suite.PreTest[0].Phase != PhasePreTest {
			t.Errorf("expected Phase PhasePreTest, got %s", suite.PreTest[0].Phase)
		}
	}

	if len(suite.PostTest) != 1 {
		t.Errorf("expected 1 PostTest step, got %d", len(suite.PostTest))
	}
	if len(suite.PostTest) > 0 {
		if suite.PostTest[0].RawText != "Collect logs." {
			t.Errorf("expected 'Collect logs.', got '%s'", suite.PostTest[0].RawText)
		}
		if suite.PostTest[0].Phase != PhasePostTest {
			t.Errorf("expected Phase PhasePostTest, got %s", suite.PostTest[0].Phase)
		}
	}

	if len(suite.PostSuite) != 2 {
		t.Errorf("expected 2 PostSuite steps, got %d", len(suite.PostSuite))
	}
	if len(suite.PostSuite) > 0 {
		if suite.PostSuite[1].RawText != "Cleanup global resources." {
			t.Errorf("expected 'Cleanup global resources.', got '%s'", suite.PostSuite[1].RawText)
		}
		if suite.PostSuite[1].Phase != PhasePostSuite {
			t.Errorf("expected Phase PhasePostSuite, got %s", suite.PostSuite[1].Phase)
		}
		if suite.PostSuite[1].Index != 1 {
			t.Errorf("expected Index 1, got %d", suite.PostSuite[1].Index)
		}
	}
}

func TestParseSuiteSpec_MinimalSpec(t *testing.T) {
	content := `# Suite: Minimal

## Pre-Test
- Setup before test.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	suite, err := ParseSuiteSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseSuiteSpec failed: %v", err)
	}

	if suite.Name != "Minimal" {
		t.Errorf("expected Name 'Minimal', got '%s'", suite.Name)
	}

	if len(suite.PreSuite) != 0 {
		t.Errorf("expected 0 PreSuite steps, got %d", len(suite.PreSuite))
	}

	if len(suite.PreTest) != 1 {
		t.Errorf("expected 1 PreTest step, got %d", len(suite.PreTest))
	}

	if len(suite.PostTest) != 0 {
		t.Errorf("expected 0 PostTest steps, got %d", len(suite.PostTest))
	}

	if len(suite.PostSuite) != 0 {
		t.Errorf("expected 0 PostSuite steps, got %d", len(suite.PostSuite))
	}
}

func TestParseSuiteSpec_NonExistentFile(t *testing.T) {
	_, err := ParseSuiteSpec("/nonexistent/suite/path.md")
	if err == nil {
		t.Fatalf("expected error for non-existent file, got nil")
	}
}

func TestParseGroupSpec_FullSpec(t *testing.T) {
	content := `# Group: with-loki

## Metadata
- Author: group-author
- Priority: Medium

## Pre-Test
1. Deploy Loki.
2. Configure logging.

## Post-Test
- Verify logs collected.
- Cleanup Loki.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	group, err := ParseGroupSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseGroupSpec failed: %v", err)
	}

	if group.Name != "with-loki" {
		t.Errorf("expected Name 'with-loki', got '%s'", group.Name)
	}

	if group.FilePath != tmpFile {
		t.Errorf("expected FilePath %s, got %s", tmpFile, group.FilePath)
	}

	if group.Metadata.Author != "group-author" {
		t.Errorf("expected Author 'group-author', got '%s'", group.Metadata.Author)
	}

	if group.Metadata.Priority != "Medium" {
		t.Errorf("expected Priority 'Medium', got '%s'", group.Metadata.Priority)
	}

	if len(group.PreTest) != 2 {
		t.Errorf("expected 2 PreTest steps, got %d", len(group.PreTest))
	}
	if len(group.PreTest) > 0 {
		if group.PreTest[0].RawText != "Deploy Loki." {
			t.Errorf("expected 'Deploy Loki.', got '%s'", group.PreTest[0].RawText)
		}
		if group.PreTest[0].Phase != PhasePreTest {
			t.Errorf("expected Phase PhasePreTest, got %s", group.PreTest[0].Phase)
		}
		if group.PreTest[0].Index != 0 {
			t.Errorf("expected Index 0, got %d", group.PreTest[0].Index)
		}
	}

	if len(group.PostTest) != 2 {
		t.Errorf("expected 2 PostTest steps, got %d", len(group.PostTest))
	}
	if len(group.PostTest) > 0 {
		if group.PostTest[0].RawText != "Verify logs collected." {
			t.Errorf("expected 'Verify logs collected.', got '%s'", group.PostTest[0].RawText)
		}
		if group.PostTest[0].Phase != PhasePostTest {
			t.Errorf("expected Phase PhasePostTest, got %s", group.PostTest[0].Phase)
		}
	}
}

func TestParseGroupSpec_MinimalSpec(t *testing.T) {
	content := `# Group: simple-group

## Post-Test
- Cleanup.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	group, err := ParseGroupSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseGroupSpec failed: %v", err)
	}

	if group.Name != "simple-group" {
		t.Errorf("expected Name 'simple-group', got '%s'", group.Name)
	}

	if len(group.PreTest) != 0 {
		t.Errorf("expected 0 PreTest steps, got %d", len(group.PreTest))
	}

	if len(group.PostTest) != 1 {
		t.Errorf("expected 1 PostTest step, got %d", len(group.PostTest))
	}
}

func TestParseGroupSpec_NonExistentFile(t *testing.T) {
	_, err := ParseGroupSpec("/nonexistent/group/path.md")
	if err == nil {
		t.Fatalf("expected error for non-existent file, got nil")
	}
}

func TestParseSteps_NumberedFormat(t *testing.T) {
	content := `# Test: Numbered Steps

## Steps
1. First step.
2. Second step.
10. Tenth step.
100. Hundredth step.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if len(spec.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(spec.Steps))
	}

	expectedSteps := []string{
		"First step.",
		"Second step.",
		"Tenth step.",
		"Hundredth step.",
	}

	for i, expected := range expectedSteps {
		if spec.Steps[i].RawText != expected {
			t.Errorf("step %d: expected '%s', got '%s'", i, expected, spec.Steps[i].RawText)
		}
	}
}

func TestParseMetadata_WithAsterisks(t *testing.T) {
	content := `# Test: Asterisk Metadata

## Metadata
* Author: test-author
* Priority: Low
* CaseID: CASE-123

## Steps
1. Test.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if spec.Metadata.Author != "test-author" {
		t.Errorf("expected Author 'test-author', got '%s'", spec.Metadata.Author)
	}

	if spec.Metadata.Priority != "Low" {
		t.Errorf("expected Priority 'Low', got '%s'", spec.Metadata.Priority)
	}

	if spec.Metadata.CaseID != "CASE-123" {
		t.Errorf("expected CaseID 'CASE-123', got '%s'", spec.Metadata.CaseID)
	}
}

func TestParseMetadata_ListFormats(t *testing.T) {
	testCases := []struct {
		name     string
		listStr  string
		expected []string
	}{
		{
			name:     "bracketed comma-separated",
			listStr:  "[a, b, c]",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "unbracketed comma-separated",
			listStr:  "x, y, z",
			expected: []string{"x", "y", "z"},
		},
		{
			name:     "single item",
			listStr:  "single",
			expected: []string{"single"},
		},
		{
			name:     "bracketed single item",
			listStr:  "[single]",
			expected: []string{"single"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := `# Test: List Test

## Metadata
- Labels: ` + tc.listStr + `

## Steps
1. Test.
`
			tmpFile := createTempFile(t, content)
			defer os.Remove(tmpFile)

			spec, err := ParseTestSpec(tmpFile)
			if err != nil {
				t.Fatalf("ParseTestSpec failed: %v", err)
			}

			if len(spec.Metadata.Labels) != len(tc.expected) {
				t.Errorf("expected %d labels, got %d", len(tc.expected), len(spec.Metadata.Labels))
				return
			}

			for i, expected := range tc.expected {
				if spec.Metadata.Labels[i] != expected {
					t.Errorf("label %d: expected '%s', got '%s'", i, expected, spec.Metadata.Labels[i])
				}
			}
		})
	}
}

func TestParseTestSpec_AllSectionsWithIndexing(t *testing.T) {
	content := `# Test: Index Test

## Setup
1. Setup step 1.
2. Setup step 2.
3. Setup step 3.

## Steps
1. Main step 1.
2. Main step 2.

## Verify
- Verify check 1.
- Verify check 2.
- Verify check 3.
- Verify check 4.

## Cleanup
1. Cleanup step 1.
`

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	// Check Setup indices
	for i := 0; i < len(spec.Setup); i++ {
		if spec.Setup[i].Index != i {
			t.Errorf("Setup[%d]: expected Index %d, got %d", i, i, spec.Setup[i].Index)
		}
	}

	// Check Steps indices
	for i := 0; i < len(spec.Steps); i++ {
		if spec.Steps[i].Index != i {
			t.Errorf("Steps[%d]: expected Index %d, got %d", i, i, spec.Steps[i].Index)
		}
	}

	// Check Verify indices
	for i := 0; i < len(spec.Verify); i++ {
		if spec.Verify[i].Index != i {
			t.Errorf("Verify[%d]: expected Index %d, got %d", i, i, spec.Verify[i].Index)
		}
	}

	// Check Cleanup indices
	for i := 0; i < len(spec.Cleanup); i++ {
		if spec.Cleanup[i].Index != i {
			t.Errorf("Cleanup[%d]: expected Index %d, got %d", i, i, spec.Cleanup[i].Index)
		}
	}
}

func TestParseTestSpec_TitleVariations(t *testing.T) {
	testCases := []struct {
		name          string
		heading       string
		expectedTitle string
	}{
		{"explicit test prefix", "# Test: My Test", "My Test"},
		{"without prefix", "# Just a title", "Just a title"},
		{"empty", "#", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := tc.heading + `

## Steps
1. Test step.
`
			tmpFile := createTempFile(t, content)
			defer os.Remove(tmpFile)

			spec, err := ParseTestSpec(tmpFile)
			if err != nil {
				t.Fatalf("ParseTestSpec failed: %v", err)
			}

			if spec.Name != tc.expectedTitle {
				t.Errorf("expected Name '%s', got '%s'", tc.expectedTitle, spec.Name)
			}
		})
	}
}

func TestParseSuiteSpec_TitleVariations(t *testing.T) {
	testCases := []struct {
		name          string
		heading       string
		expectedTitle string
	}{
		{"explicit suite prefix", "# Suite: My Suite", "My Suite"},
		{"without prefix", "# Just a suite", "Just a suite"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := tc.heading + `

## Pre-Test
- Setup.
`
			tmpFile := createTempFile(t, content)
			defer os.Remove(tmpFile)

			suite, err := ParseSuiteSpec(tmpFile)
			if err != nil {
				t.Fatalf("ParseSuiteSpec failed: %v", err)
			}

			if suite.Name != tc.expectedTitle {
				t.Errorf("expected Name '%s', got '%s'", tc.expectedTitle, suite.Name)
			}
		})
	}
}

func TestParseGroupSpec_TitleVariations(t *testing.T) {
	testCases := []struct {
		name          string
		heading       string
		expectedTitle string
	}{
		{"explicit group prefix", "# Group: my-group", "my-group"},
		{"without prefix", "# Just a group", "Just a group"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := tc.heading + `

## Pre-Test
- Setup.
`
			tmpFile := createTempFile(t, content)
			defer os.Remove(tmpFile)

			group, err := ParseGroupSpec(tmpFile)
			if err != nil {
				t.Fatalf("ParseGroupSpec failed: %v", err)
			}

			if group.Name != tc.expectedTitle {
				t.Errorf("expected Name '%s', got '%s'", tc.expectedTitle, group.Name)
			}
		})
	}
}

func TestParseTestSpec_EmptyFile(t *testing.T) {
	content := ""
	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	spec, err := ParseTestSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseTestSpec failed: %v", err)
	}

	if spec.Name != "" {
		t.Errorf("expected empty Name, got '%s'", spec.Name)
	}

	if len(spec.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(spec.Steps))
	}
}

// Helper function to create temporary files
func createTempFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "spec_test_*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	return tmpFile.Name()
}
