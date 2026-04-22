package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/openshift/sdt/pkg/cache"
	"github.com/openshift/sdt/pkg/llm"
	"github.com/openshift/sdt/pkg/template"
)

// CodingAgent generates and validates YAML templates using the LLM.
type CodingAgent struct {
	llmClient *llm.Client
	registry  *template.TemplateRegistry
	store     *cache.Store
}

// NewCodingAgent creates a new coding agent.
func NewCodingAgent(llmClient *llm.Client, registry *template.TemplateRegistry, store *cache.Store) *CodingAgent {
	return &CodingAgent{
		llmClient: llmClient,
		registry:  registry,
		store:     store,
	}
}

// GenerateTemplate generates a YAML template based on a description.
// Uses cache to avoid regenerating the same template.
// references can contain names of existing templates to use as style examples.
func (c *CodingAgent) GenerateTemplate(ctx context.Context, description string, references []string) (string, error) {
	// Compute hash of description for caching
	descHash := hashDescription(description)

	// Check cache first
	if cached, ok := c.store.GetGeneratedTemplate(descHash); ok {
		return string(cached), nil
	}

	// Build reference examples
	refExamples := ""
	for _, refName := range references {
		if def := c.registry.Get(refName); def != nil {
			// Try to read the referenced template
			// For now, just note the reference
			refExamples += fmt.Sprintf("\n- Reference template: %s (%s)\n", refName, def.Description)
		}
	}

	prompt := fmt.Sprintf(`Generate a YAML template for the following:

Description:
%s

%s

Generate the template as valid YAML. Use template parameters ({{.param}}) for configurable values.
Include appropriate labels and metadata. Output only the YAML template, no explanations.
`, description, refExamples)

	resp, err := c.llmClient.Chat(ctx, CodingSystemPrompt, prompt, nil)
	if err != nil {
		return "", fmt.Errorf("llm call failed: %w", err)
	}

	content := resp.TextContent()

	// Validate the template
	if err := template.ValidateTemplate(content); err != nil {
		return "", fmt.Errorf("generated template is not valid YAML: %w", err)
	}

	// Save to cache
	c.store.SaveGeneratedTemplate(descHash, []byte(content), nil)

	return content, nil
}

// ValidateTemplate validates a YAML template for correctness.
// Checks that it's valid YAML and represents a Kubernetes resource.
func (c *CodingAgent) ValidateTemplate(ctx context.Context, content string, kind string) error {
	// Basic validation
	if err := template.ValidateTemplate(content); err != nil {
		return fmt.Errorf("template is not valid YAML: %w", err)
	}

	// Use LLM to do deeper validation
	prompt := fmt.Sprintf(`Validate this YAML template:

Kind: %s

Template:
%s

Check:
1. Is it valid YAML?
2. Does it define the intended Kubernetes resource (%s)?
3. Are all parameters properly defined?
4. Is indentation correct?
5. Are there any obvious errors or issues?

Respond with:
- VALID if the template is acceptable
- INVALID followed by the specific issues if there are problems
`, kind, content, kind)

	resp, err := c.llmClient.Chat(ctx, ValidatorSystemPrompt, prompt, nil)
	if err != nil {
		return fmt.Errorf("validation call failed: %w", err)
	}

	respText := resp.TextContent()
	if len(respText) > 0 && respText[0:5] != "VALID" {
		return fmt.Errorf("validation failed: %s", respText)
	}

	return nil
}

// RegenerateTemplate regenerates a template by clearing its cache and generating again.
func (c *CodingAgent) RegenerateTemplate(ctx context.Context, description string, references []string) (string, error) {
	descHash := hashDescription(description)
	c.store.InvalidateTemplate(descHash)
	return c.GenerateTemplate(ctx, description, references)
}

// hashDescription computes a SHA256 hash of the template description.
func hashDescription(desc string) string {
	hash := sha256.Sum256([]byte(desc))
	return hex.EncodeToString(hash[:])
}
