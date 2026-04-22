package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openshift/sdt/pkg/log"
)

// OCClient wraps the `oc` CLI binary for cluster interaction.
type OCClient struct {
	kubeconfig string
	ocPath     string
}

// NewOCClient creates a new OC client, auto-detecting oc binary and kubeconfig.
func NewOCClient() (*OCClient, error) {
	ocPath, err := exec.LookPath("oc")
	if err != nil {
		return nil, fmt.Errorf("oc binary not found in PATH: %w", err)
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	return &OCClient{
		kubeconfig: kubeconfig,
		ocPath:     ocPath,
	}, nil
}

// Run executes an oc command and returns stdout, stderr.
func (c *OCClient) Run(ctx context.Context, args ...string) (string, string, error) {
	log.Debugf("OC", "oc %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, c.ocPath, args...)
	if c.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+c.kubeconfig)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		log.Debugf("OC", "oc %s failed after %s: %v", args[0], elapsed, err)
	} else {
		log.Debugf("OC", "oc %s completed in %s", args[0], elapsed)
	}

	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// RunWithStdin executes an oc command with data piped to stdin.
func (c *OCClient) RunWithStdin(ctx context.Context, stdinData string, args ...string) (string, string, error) {
	log.Debugf("OC", "oc %s (with stdin, %d bytes)", strings.Join(args, " "), len(stdinData))
	cmd := exec.CommandContext(ctx, c.ocPath, args...)
	if c.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+c.kubeconfig)
	}

	cmd.Stdin = strings.NewReader(stdinData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		log.Debugf("OC", "oc %s failed after %s: %v", args[0], elapsed, err)
	} else {
		log.Debugf("OC", "oc %s completed in %s", args[0], elapsed)
	}

	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// RegisterOCTools registers all oc-based MCP tools into the registry.
func RegisterOCTools(registry *Registry, oc *OCClient, constraints *ToolConstraints) {
	registry.Register(&Tool{
		Name:        "oc_run",
		Description: "Run any oc command with the given arguments. Use this ONLY for ad-hoc operations not covered by other tools. Do NOT use for: get (use oc_get), apply (use oc_apply), delete (use oc_delete), patch (use oc_patch), logs (use oc_logs), exec (use oc_exec), wait (use wait_for_condition).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"args": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Arguments to pass to oc (e.g., [\"get\", \"pods\", \"-n\", \"default\"])"
				}
			},
			"required": ["args"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Args []string `json:"args"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			cmd := strings.Join(params.Args, " ")
			if err := constraints.Check("oc_run", cmd); err != nil {
				return &ToolResult{Error: err}, nil
			}
			log.Infof("TOOL", "oc_run: oc %s", cmd)
			stdout, stderr, err := oc.Run(ctx, params.Args...)
			output := stdout
			if stderr != "" {
				output += "\nSTDERR: " + stderr
			}
			if err != nil {
				log.Warnf("TOOL", "oc_run failed: %v", err)
				return &ToolResult{Output: output, Error: fmt.Errorf("oc command failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: output}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "oc_apply",
		Description: "Apply a YAML resource file or template to a namespace. Equivalent to 'oc apply -f <file> -n <namespace>'.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"file": {
					"type": "string",
					"description": "Path to the YAML file to apply"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace to apply the resource in (optional)"
				}
			},
			"required": ["file"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				File      string `json:"file"`
				Namespace string `json:"namespace"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			args := []string{"apply", "-f", params.File}
			if params.Namespace != "" {
				args = append(args, "-n", params.Namespace)
			}
			log.Infof("TOOL", "oc_apply: file=%s namespace=%s", params.File, params.Namespace)
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "oc_apply failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("oc apply failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "oc_delete",
		Description: "Delete a Kubernetes resource by kind, name, and namespace.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"kind": {
					"type": "string",
					"description": "Resource kind (e.g., pod, deployment, flowcollector)"
				},
				"name": {
					"type": "string",
					"description": "Resource name"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace (optional for cluster-scoped resources)"
				},
				"wait": {
					"type": "boolean",
					"description": "Wait for deletion to complete (default: true)"
				}
			},
			"required": ["kind", "name"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Wait      *bool  `json:"wait"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			args := []string{"delete", params.Kind, params.Name}
			if params.Namespace != "" {
				args = append(args, "-n", params.Namespace)
			}
			if params.Wait == nil || *params.Wait {
				args = append(args, "--wait=true")
			}
			args = append(args, "--ignore-not-found")
			log.Infof("TOOL", "oc_delete: %s/%s namespace=%s", params.Kind, params.Name, params.Namespace)
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "oc_delete failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("oc delete failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "oc_get",
		Description: "Get a Kubernetes resource. Supports jsonpath output for extracting specific fields.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"kind": {
					"type": "string",
					"description": "Resource kind (e.g., pod, deployment, flowcollector)"
				},
				"name": {
					"type": "string",
					"description": "Resource name (optional, omit to list all)"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace (optional)"
				},
				"output": {
					"type": "string",
					"description": "Output format: json, yaml, wide, name, or jsonpath=<template>"
				}
			},
			"required": ["kind"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Output    string `json:"output"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			args := []string{"get", params.Kind}
			if params.Name != "" {
				args = append(args, params.Name)
			}
			if params.Namespace != "" {
				args = append(args, "-n", params.Namespace)
			}
			if params.Output != "" {
				args = append(args, "-o", params.Output)
			}
			log.Infof("TOOL", "oc_get: %s %s namespace=%s", params.Kind, params.Name, params.Namespace)
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "oc_get failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("oc get failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "oc_patch",
		Description: "Patch a Kubernetes resource.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"kind": {
					"type": "string",
					"description": "Resource kind"
				},
				"name": {
					"type": "string",
					"description": "Resource name"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace (optional)"
				},
				"patch_type": {
					"type": "string",
					"description": "Patch type: merge, json, strategic (default: merge)"
				},
				"patch": {
					"type": "string",
					"description": "JSON patch content"
				}
			},
			"required": ["kind", "name", "patch"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				PatchType string `json:"patch_type"`
				Patch     string `json:"patch"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			patchType := params.PatchType
			if patchType == "" {
				patchType = "merge"
			}
			args := []string{"patch", params.Kind, params.Name, "--type", patchType, "-p", params.Patch}
			if params.Namespace != "" {
				args = append(args, "-n", params.Namespace)
			}
			log.Infof("TOOL", "oc_patch: %s/%s namespace=%s type=%s", params.Kind, params.Name, params.Namespace, patchType)
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "oc_patch failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("oc patch failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "oc_exec",
		Description: "Execute a command inside a running pod.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pod": {
					"type": "string",
					"description": "Pod name"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace"
				},
				"container": {
					"type": "string",
					"description": "Container name (optional)"
				},
				"command": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Command and arguments to execute"
				}
			},
			"required": ["pod", "namespace", "command"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Pod       string   `json:"pod"`
				Namespace string   `json:"namespace"`
				Container string   `json:"container"`
				Command   []string `json:"command"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			args := []string{"exec", params.Pod, "-n", params.Namespace}
			if params.Container != "" {
				args = append(args, "-c", params.Container)
			}
			args = append(args, "--")
			args = append(args, params.Command...)
			log.Infof("TOOL", "oc_exec: pod=%s namespace=%s command=%s", params.Pod, params.Namespace, strings.Join(params.Command, " "))
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "oc_exec failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("oc exec failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "oc_logs",
		Description: "Get logs from a pod.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pod": {
					"type": "string",
					"description": "Pod name"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace"
				},
				"container": {
					"type": "string",
					"description": "Container name (optional)"
				},
				"tail": {
					"type": "integer",
					"description": "Number of lines from the end to show (default: 100)"
				},
				"previous": {
					"type": "boolean",
					"description": "Show logs from previous container instance"
				}
			},
			"required": ["pod", "namespace"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Pod       string `json:"pod"`
				Namespace string `json:"namespace"`
				Container string `json:"container"`
				Tail      int    `json:"tail"`
				Previous  bool   `json:"previous"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}
			args := []string{"logs", params.Pod, "-n", params.Namespace}
			if params.Container != "" {
				args = append(args, "-c", params.Container)
			}
			tail := params.Tail
			if tail == 0 {
				tail = 100
			}
			args = append(args, fmt.Sprintf("--tail=%d", tail))
			if params.Previous {
				args = append(args, "--previous")
			}
			log.Infof("TOOL", "oc_logs: pod=%s namespace=%s tail=%d", params.Pod, params.Namespace, tail)
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "oc_logs failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("oc logs failed: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})
}
