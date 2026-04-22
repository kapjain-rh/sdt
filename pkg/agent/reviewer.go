package agent

import (
	"context"
	"fmt"

	"github.com/openshift/sdt/pkg/llm"
	"github.com/openshift/sdt/pkg/spec"
)

// ReviewerAgent reviews test specifications and analyzes execution failures.
type ReviewerAgent struct {
	llmClient *llm.Client
}

// ReviewResult holds the outcome of a review or analysis.
type ReviewResult struct {
	Score       int      `json:"score"`       // 0-100
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
	Details     string   `json:"details,omitempty"`
}

// NewReviewerAgent creates a new reviewer agent.
func NewReviewerAgent(llmClient *llm.Client) *ReviewerAgent {
	return &ReviewerAgent{
		llmClient: llmClient,
	}
}

// ReviewSpec reviews the quality of a test specification.
// It checks for completeness, clarity, and best practices.
func (r *ReviewerAgent) ReviewSpec(ctx context.Context, testSpec *spec.TestSpec) (*ReviewResult, error) {
	prompt := buildSpecReviewPrompt(testSpec)

	resp, err := r.llmClient.Chat(ctx, ReviewerSystemPrompt, prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	// Parse response into ReviewResult
	// For now, create a simple result based on response
	result := &ReviewResult{
		Details: resp.TextContent(),
		Score:   75, // Default score
	}

	return result, nil
}

// AnalyzeFailure analyzes an execution failure and identifies root causes.
func (r *ReviewerAgent) AnalyzeFailure(ctx context.Context, testSpec *spec.TestSpec, executionResult *ExecutionResult) (*ReviewResult, error) {
	prompt := buildFailureAnalysisPrompt(testSpec, executionResult)

	resp, err := r.llmClient.Chat(ctx, ReviewerSystemPrompt, prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	// Parse response into ReviewResult
	result := &ReviewResult{
		Details: resp.TextContent(),
		Score:   0, // Failures get 0 score
	}

	return result, nil
}

// buildSpecReviewPrompt constructs the prompt for spec review.
func buildSpecReviewPrompt(testSpec *spec.TestSpec) string {
	prompt := fmt.Sprintf(`Review the following test specification for quality and completeness:

Test Name: %s
File: %s
Author: %s
Priority: %s
Case ID: %s
Labels: %v
Timeout: %v

Description: %s

Setup steps: %d
Test steps: %d
Verification steps: %d
Cleanup steps: %d

Please evaluate:
1. Is the metadata complete and accurate?
2. Are the test steps clear and unambiguous?
3. Are there missing or incomplete steps?
4. Is the test timeout appropriate?
5. Are verification steps adequate?
6. Are cleanup steps proper?
7. Would this test be easy to maintain?

Provide a score (0-100) and list any issues or suggestions for improvement.
`, testSpec.Name, testSpec.FilePath, testSpec.Metadata.Author, testSpec.Metadata.Priority,
	testSpec.Metadata.CaseID, testSpec.Metadata.Labels, testSpec.Metadata.Timeout,
	testSpec.Name, len(testSpec.Setup), len(testSpec.Steps), len(testSpec.Verify), len(testSpec.Cleanup))

	return prompt
}

// buildFailureAnalysisPrompt constructs the prompt for failure analysis.
func buildFailureAnalysisPrompt(testSpec *spec.TestSpec, result *ExecutionResult) string {
	prompt := fmt.Sprintf(`Analyze the failure of this test and identify root causes:

Test: %s
Status: %s
Duration: %s
Error: %s

Tool Calls Made:
`, testSpec.Name, result.Status, result.Duration, result.Error)

	for i, log := range result.ToolCalls {
		prompt += fmt.Sprintf("%d. Tool: %s\n", i+1, log.ToolName)
		if log.Error != "" {
			prompt += fmt.Sprintf("   Error: %s\n", log.Error)
		} else {
			prompt += fmt.Sprintf("   Output: %s\n", log.Output)
		}
	}

	prompt += `
Please analyze:
1. Which step failed first?
2. What is the root cause of the failure?
3. Is this a cluster/environment issue, test design issue, or timing issue?
4. What are possible fixes?
5. Should the test be modified, or is the cluster misconfigured?

Provide detailed analysis and recommendations.`

	return prompt
}
