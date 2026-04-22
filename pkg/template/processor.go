package template

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

// CommandRunner is the interface needed to run CLI commands for template processing.
type CommandRunner interface {
	Run(ctx context.Context, args ...string) (stdout string, stderr string, err error)
}

// Processor handles template rendering and processing.
type Processor struct {
	runner CommandRunner
}

// NewProcessor creates a new template processor.
func NewProcessor(runner CommandRunner) *Processor {
	return &Processor{
		runner: runner,
	}
}

// ProcessOCTemplate runs `oc process` on an OpenShift template file with the given parameters.
// Returns the processed YAML as a string.
func (p *Processor) ProcessOCTemplate(ctx context.Context, templatePath string, params map[string]string, namespace string) (string, error) {
	args := []string{"process", "-f", templatePath}

	// Add parameters to the command
	for key, value := range params {
		args = append(args, "-p", fmt.Sprintf("%s=%s", key, value))
	}

	// Add namespace if provided
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	stdout, stderr, err := p.runner.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("oc process failed: %w\nstderr: %s", err, stderr)
	}

	return stdout, nil
}

// RenderGoTemplate renders a Go text/template file with the given data.
// The data map is passed to the template as context.
func (p *Processor) RenderGoTemplate(templatePath string, data map[string]string) (string, error) {
	// Read the template file
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template file: %w", err)
	}

	// Parse the template
	tmpl, err := template.New(templatePath).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	// Render the template with the data
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// ProcessRawTemplate processes a raw template string (either Go template or OpenShift template syntax)
// and returns the processed result.
func (p *Processor) ProcessRawTemplate(ctx context.Context, content string, params map[string]string, isGoTemplate bool) (string, error) {
	if isGoTemplate {
		// Parse as Go template
		tmpl, err := template.New("raw").Parse(content)
		if err != nil {
			return "", fmt.Errorf("parsing Go template: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, params); err != nil {
			return "", fmt.Errorf("executing Go template: %w", err)
		}

		return buf.String(), nil
	}

	// For OpenShift templates, write to a temp file and process via oc
	tmpFile, err := os.CreateTemp("", "template-*.yaml")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("writing template to temp file: %w", err)
	}
	tmpFile.Close()

	return p.ProcessOCTemplate(ctx, tmpFile.Name(), params, "")
}

// ValidateTemplate checks if a YAML template is valid by parsing it.
func ValidateTemplate(content string) error {
	// Try to unmarshal as JSON (YAML is a superset of JSON)
	var obj interface{}
	return json.Unmarshal([]byte(content), &obj)
}

// ExecuteCommand is a helper to execute arbitrary commands (e.g., kustomize, helm).
// This is used by agents that need to process templates with external tools.
func ExecuteCommand(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
