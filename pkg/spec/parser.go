package spec

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// parserState tracks which section the parser is currently in.
type parserState int

const (
	stateNone parserState = iota
	stateMetadata
	statePreSuite
	statePreSuiteValidation
	statePreTest
	statePreTestValidation
	stateSetup
	stateSteps
	stateVerify
	stateCleanup
	statePostTest
	statePostSuite
)

// ParseTestSpec parses a test spec markdown file into a TestSpec.
func ParseTestSpec(filePath string) (*TestSpec, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading spec file %s: %w", filePath, err)
	}

	spec := &TestSpec{FilePath: filePath}
	state := stateNone
	var stepCounters = map[parserState]int{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect headings
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			spec.Name = extractTitle(trimmed, "# Test:")
			if spec.Name == "" {
				spec.Name = strings.TrimPrefix(trimmed, "# ")
			}
			continue
		}

		newState, matched := matchSectionHeading(trimmed)
		if matched {
			state = newState
			continue
		}

		// Parse content based on current state
		switch state {
		case stateMetadata:
			parseMetadataLine(trimmed, &spec.Metadata)
		case stateSetup:
			cnt := stepCounters[stateSetup]
			spec.Setup = appendStep(spec.Setup, trimmed, PhaseSetup, &cnt)
			stepCounters[stateSetup] = cnt
		case stateSteps:
			cnt := stepCounters[stateSteps]
			spec.Steps = appendStep(spec.Steps, trimmed, PhaseSteps, &cnt)
			stepCounters[stateSteps] = cnt
		case stateVerify:
			cnt := stepCounters[stateVerify]
			spec.Verify = appendStep(spec.Verify, trimmed, PhaseVerify, &cnt)
			stepCounters[stateVerify] = cnt
		case stateCleanup:
			cnt := stepCounters[stateCleanup]
			spec.Cleanup = appendStep(spec.Cleanup, trimmed, PhaseCleanup, &cnt)
			stepCounters[stateCleanup] = cnt
		}
	}

	return spec, nil
}

// ParseSuiteSpec parses a _suite.md file into a SuiteSpec.
func ParseSuiteSpec(filePath string) (*SuiteSpec, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading suite file %s: %w", filePath, err)
	}

	suite := &SuiteSpec{FilePath: filePath}
	state := stateNone
	var stepCounters = map[parserState]int{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			suite.Name = extractTitle(trimmed, "# Suite:")
			if suite.Name == "" {
				suite.Name = strings.TrimPrefix(trimmed, "# ")
			}
			continue
		}

		newState, matched := matchSectionHeading(trimmed)
		if matched {
			state = newState
			continue
		}

		switch state {
		case stateMetadata:
			parseMetadataLine(trimmed, &suite.Metadata)
		case statePreSuite:
			cnt := stepCounters[statePreSuite]
			suite.PreSuite = appendStep(suite.PreSuite, trimmed, PhasePreSuite, &cnt)
			stepCounters[statePreSuite] = cnt
		case statePreSuiteValidation:
			cnt := stepCounters[statePreSuiteValidation]
			suite.PreSuiteValidation = appendStep(suite.PreSuiteValidation, trimmed, PhasePreSuiteValidation, &cnt)
			stepCounters[statePreSuiteValidation] = cnt
		case statePreTest:
			cnt := stepCounters[statePreTest]
			suite.PreTest = appendStep(suite.PreTest, trimmed, PhasePreTest, &cnt)
			stepCounters[statePreTest] = cnt
		case statePreTestValidation:
			cnt := stepCounters[statePreTestValidation]
			suite.PreTestValidation = appendStep(suite.PreTestValidation, trimmed, PhasePreTestValidation, &cnt)
			stepCounters[statePreTestValidation] = cnt
		case statePostTest:
			cnt := stepCounters[statePostTest]
			suite.PostTest = appendStep(suite.PostTest, trimmed, PhasePostTest, &cnt)
			stepCounters[statePostTest] = cnt
		case statePostSuite:
			cnt := stepCounters[statePostSuite]
			suite.PostSuite = appendStep(suite.PostSuite, trimmed, PhasePostSuite, &cnt)
			stepCounters[statePostSuite] = cnt
		}
	}

	return suite, nil
}

// ParseGroupSpec parses a _group_<name>.md file into a GroupSpec.
func ParseGroupSpec(filePath string) (*GroupSpec, error) {
	lines, err := readLines(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading group file %s: %w", filePath, err)
	}

	group := &GroupSpec{FilePath: filePath}
	state := stateNone
	var stepCounters = map[parserState]int{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			group.Name = extractTitle(trimmed, "# Group:")
			if group.Name == "" {
				group.Name = strings.TrimPrefix(trimmed, "# ")
			}
			continue
		}

		newState, matched := matchSectionHeading(trimmed)
		if matched {
			state = newState
			continue
		}

		switch state {
		case stateMetadata:
			parseMetadataLine(trimmed, &group.Metadata)
		case statePreTest:
			cnt := stepCounters[statePreTest]
			group.PreTest = appendStep(group.PreTest, trimmed, PhasePreTest, &cnt)
			stepCounters[statePreTest] = cnt
		case statePreTestValidation:
			cnt := stepCounters[statePreTestValidation]
			group.PreTestValidation = appendStep(group.PreTestValidation, trimmed, PhasePreTestValidation, &cnt)
			stepCounters[statePreTestValidation] = cnt
		case statePostTest:
			cnt := stepCounters[statePostTest]
			group.PostTest = appendStep(group.PostTest, trimmed, PhasePostTest, &cnt)
			stepCounters[statePostTest] = cnt
		}
	}

	return group, nil
}

// matchSectionHeading checks if a line is a ## heading and returns the corresponding state.
func matchSectionHeading(line string) (parserState, bool) {
	if !strings.HasPrefix(line, "## ") {
		return stateNone, false
	}
	heading := strings.ToLower(strings.TrimPrefix(line, "## "))
	heading = strings.TrimSpace(heading)
	switch heading {
	case "metadata":
		return stateMetadata, true
	case "pre-suite":
		return statePreSuite, true
	case "pre-suite validation":
		return statePreSuiteValidation, true
	case "pre-test":
		return statePreTest, true
	case "pre-test validation":
		return statePreTestValidation, true
	case "setup":
		return stateSetup, true
	case "steps":
		return stateSteps, true
	case "verify":
		return stateVerify, true
	case "cleanup":
		return stateCleanup, true
	case "post-test":
		return statePostTest, true
	case "post-suite":
		return statePostSuite, true
	default:
		return stateNone, false
	}
}

// parseMetadataLine parses a "- Key: Value" line into metadata fields.
func parseMetadataLine(line string, m *Metadata) {
	if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
		return
	}
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(strings.ToLower(parts[0]))
	value := strings.TrimSpace(parts[1])

	switch key {
	case "author":
		m.Author = value
	case "priority":
		m.Priority = value
	case "caseid":
		m.CaseID = value
	case "labels":
		m.Labels = parseList(value)
	case "timeout":
		if d, err := time.ParseDuration(value); err == nil {
			m.Timeout = d
		}
	case "group":
		m.Group = value
	case "fixtures":
		m.Fixtures = parseList(value)
	}
}

// parseList parses "[a, b, c]" or "a, b, c" into a string slice.
func parseList(s string) []string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseStepLine extracts step text from "- text" or "N. text" formatted lines.
// Returns (step, isNewStep=true) for lines that start a new step.
// Returns (StepDef{RawText: line}, isNewStep=false) for continuation lines.
// Returns (StepDef{}, false) for blank lines.
func parseStepLine(line string) (step StepDef, isNewStep bool) {
	if line == "" {
		return StepDef{}, false
	}
	// Bullet list item
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		text := strings.TrimPrefix(line, "- ")
		text = strings.TrimPrefix(text, "* ")
		return StepDef{RawText: text}, true
	}
	// Numbered list item (e.g., "1. text", "12. text")
	for i, c := range line {
		if c == '.' && i > 0 {
			prefix := line[:i]
			allDigits := true
			for _, d := range prefix {
				if d < '0' || d > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && i+1 < len(line) && line[i+1] == ' ' {
				return StepDef{RawText: strings.TrimSpace(line[i+2:])}, true
			}
			break
		}
		if c < '0' || c > '9' {
			break
		}
	}
	// Continuation line — return the text but mark as not a new step
	return StepDef{RawText: line}, false
}

// appendStep adds a new step or appends continuation text to the last step in the slice.
func appendStep(steps []StepDef, trimmed string, phase Phase, counter *int) []StepDef {
	step, isNew := parseStepLine(trimmed)
	if step.RawText == "" {
		return steps
	}
	if isNew {
		step.Phase = phase
		step.Index = *counter
		*counter++
		return append(steps, step)
	}
	// Continuation line — append to the previous step
	if len(steps) > 0 {
		steps[len(steps)-1].RawText += " " + step.RawText
	}
	return steps
}

// extractTitle pulls the title from a heading like "# Test: My Title" or "# Suite: My Suite".
func extractTitle(line, prefix string) string {
	if strings.HasPrefix(line, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}
	return ""
}

// readLines reads a file and returns its lines.
func readLines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
