package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/sdt/pkg/log"
)

// RegisterResourceTools registers higher-level resource management tools.
func RegisterResourceTools(registry *Registry, oc *OCClient) {
	registry.Register(&Tool{
		Name:        "create_namespace",
		Description: "Create a Kubernetes namespace. Safe to call if namespace already exists.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Namespace name to create"
				},
				"labels": {
					"type": "object",
					"additionalProperties": {"type": "string"},
					"description": "Labels to apply to the namespace (optional)"
				}
			},
			"required": ["name"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			log.Infof("TOOL", "create_namespace: %s", params.Name)

			// Check if namespace already exists
			stdout, _, err := oc.Run(ctx, "get", "namespace", params.Name, "--ignore-not-found")
			if err == nil && strings.Contains(stdout, params.Name) {
				log.Infof("TOOL", "create_namespace: %s already exists", params.Name)
				return &ToolResult{Output: fmt.Sprintf("Namespace %s already exists", params.Name)}, nil
			}

			args := []string{"create", "namespace", params.Name}
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "create_namespace failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("creating namespace: %w\n%s", err, stderr)}, nil
			}

			// Apply labels if provided
			if len(params.Labels) > 0 {
				for k, v := range params.Labels {
					_, stderr, err := oc.Run(ctx, "label", "namespace", params.Name, fmt.Sprintf("%s=%s", k, v), "--overwrite")
					if err != nil {
						return &ToolResult{Output: stdout, Error: fmt.Errorf("labeling namespace: %w\n%s", err, stderr)}, nil
					}
				}
			}

			log.Infof("TOOL", "create_namespace: %s created", params.Name)
			return &ToolResult{Output: fmt.Sprintf("Namespace %s created", params.Name)}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "delete_namespace",
		Description: "Delete a Kubernetes namespace and wait for termination.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Namespace name to delete"
				}
			},
			"required": ["name"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			log.Infof("TOOL", "delete_namespace: %s", params.Name)
			stdout, stderr, err := oc.Run(ctx, "delete", "namespace", params.Name, "--ignore-not-found", "--wait=true")
			if err != nil {
				log.Warnf("TOOL", "delete_namespace failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("deleting namespace: %w\n%s", err, stderr)}, nil
			}
			log.Infof("TOOL", "delete_namespace: %s deleted", params.Name)
			return &ToolResult{Output: fmt.Sprintf("Namespace %s deleted", params.Name)}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "wait_for_condition",
		Description: "Wait until a resource reaches a specific condition (e.g., Ready, Available). Polls periodically until the condition is met or timeout expires.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"kind": {
					"type": "string",
					"description": "Resource kind (e.g., deployment, pod, flowcollector)"
				},
				"name": {
					"type": "string",
					"description": "Resource name"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace (optional for cluster-scoped resources)"
				},
				"condition": {
					"type": "string",
					"description": "Condition to wait for (e.g., Ready, Available)"
				},
				"jsonpath": {
					"type": "string",
					"description": "JSONPath expression to check (alternative to condition)"
				},
				"expected": {
					"type": "string",
					"description": "Expected value for the jsonpath expression"
				},
				"timeout": {
					"type": "string",
					"description": "Timeout duration (e.g., 300s, 10m). Default: 300s"
				}
			},
			"required": ["kind", "name"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Condition string `json:"condition"`
				JSONPath  string `json:"jsonpath"`
				Expected  string `json:"expected"`
				Timeout   string `json:"timeout"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			timeout := params.Timeout
			if timeout == "" {
				timeout = "300s"
			}
			timeoutDur, err := time.ParseDuration(timeout)
			if err != nil {
				timeoutDur = 300 * time.Second
			}

			if params.Condition != "" {
				log.Infof("TOOL", "wait_for_condition: %s/%s condition=%s timeout=%s namespace=%s",
					params.Kind, params.Name, params.Condition, timeoutDur, params.Namespace)
				// Use oc wait with condition
				args := []string{"wait", fmt.Sprintf("%s/%s", params.Kind, params.Name),
					fmt.Sprintf("--for=condition=%s", params.Condition),
					fmt.Sprintf("--timeout=%s", timeoutDur)}
				if params.Namespace != "" {
					args = append(args, "-n", params.Namespace)
				}
				start := time.Now()
				stdout, stderr, err := oc.Run(ctx, args...)
				if err != nil {
					log.Warnf("TOOL", "wait_for_condition: %s/%s condition=%s FAILED after %s: %v",
						params.Kind, params.Name, params.Condition, time.Since(start), err)
					return &ToolResult{Output: stdout, Error: fmt.Errorf("wait for condition failed: %w\n%s", err, stderr)}, nil
				}
				log.Infof("TOOL", "wait_for_condition: %s/%s condition=%s met after %s",
					params.Kind, params.Name, params.Condition, time.Since(start))
				return &ToolResult{Output: stdout}, nil
			}

			// Poll with jsonpath
			if params.JSONPath == "" {
				return nil, fmt.Errorf("either condition or jsonpath must be specified")
			}

			log.Infof("TOOL", "wait_for_condition: %s/%s jsonpath=%s expected=%s timeout=%s namespace=%s",
				params.Kind, params.Name, params.JSONPath, params.Expected, timeoutDur, params.Namespace)

			deadline := time.Now().Add(timeoutDur)
			pollStart := time.Now()
			polls := 0
			for time.Now().Before(deadline) {
				polls++
				args := []string{"get", params.Kind, params.Name, "-o", fmt.Sprintf("jsonpath=%s", params.JSONPath)}
				if params.Namespace != "" {
					args = append(args, "-n", params.Namespace)
				}
				stdout, _, err := oc.Run(ctx, args...)
				if err == nil && strings.Contains(stdout, params.Expected) {
					log.Infof("TOOL", "wait_for_condition: %s/%s jsonpath condition met after %s (%d polls)",
						params.Kind, params.Name, time.Since(pollStart), polls)
					return &ToolResult{Output: fmt.Sprintf("Condition met: %s = %s", params.JSONPath, stdout)}, nil
				}

				remaining := time.Until(deadline)
				log.Debugf("TOOL", "wait_for_condition: %s/%s poll %d — current=%q expected=%q remaining=%s",
					params.Kind, params.Name, polls, stdout, params.Expected, remaining.Truncate(time.Second))

				select {
				case <-ctx.Done():
					log.Warnf("TOOL", "wait_for_condition: %s/%s context cancelled after %s (%d polls)",
						params.Kind, params.Name, time.Since(pollStart), polls)
					return &ToolResult{Error: ctx.Err()}, nil
				case <-time.After(10 * time.Second):
				}
			}

			log.Errorf("TOOL", "wait_for_condition: %s/%s TIMEOUT after %s (%d polls) — jsonpath=%s expected=%s",
				params.Kind, params.Name, time.Since(pollStart), polls, params.JSONPath, params.Expected)
			return &ToolResult{Error: fmt.Errorf("timeout waiting for %s/%s %s=%s", params.Kind, params.Name, params.JSONPath, params.Expected)}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "wait_for_pods_ready",
		Description: "Wait until all pods matching a label selector or in a namespace are Ready and Running.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"namespace": {
					"type": "string",
					"description": "Namespace to check pods in"
				},
				"label": {
					"type": "string",
					"description": "Label selector (e.g., app=netobserv). Optional."
				},
				"timeout": {
					"type": "string",
					"description": "Timeout duration (e.g., 300s, 10m). Default: 300s"
				}
			},
			"required": ["namespace"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Namespace string `json:"namespace"`
				Label     string `json:"label"`
				Timeout   string `json:"timeout"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			timeout := params.Timeout
			if timeout == "" {
				timeout = "300s"
			}
			timeoutDur, err := time.ParseDuration(timeout)
			if err != nil {
				timeoutDur = 300 * time.Second
			}

			log.Infof("TOOL", "wait_for_pods_ready: namespace=%s label=%s timeout=%s",
				params.Namespace, params.Label, timeoutDur)

			deadline := time.Now().Add(timeoutDur)
			pollStart := time.Now()
			polls := 0
			for time.Now().Before(deadline) {
				polls++
				args := []string{"get", "pods", "-n", params.Namespace, "-o", "jsonpath={.items[*].status.phase}"}
				if params.Label != "" {
					args = append(args, "-l", params.Label)
				}
				stdout, _, err := oc.Run(ctx, args...)
				if err == nil && stdout != "" {
					phases := strings.Fields(stdout)
					allRunning := len(phases) > 0
					notReady := 0
					for _, p := range phases {
						if p != "Running" && p != "Succeeded" {
							allRunning = false
							notReady++
						}
					}
					if allRunning {
						log.Infof("TOOL", "wait_for_pods_ready: all %d pods ready in %s after %s (%d polls)",
							len(phases), params.Namespace, time.Since(pollStart), polls)
						return &ToolResult{Output: fmt.Sprintf("All %d pods are Ready in namespace %s", len(phases), params.Namespace)}, nil
					}
					log.Debugf("TOOL", "wait_for_pods_ready: namespace=%s — %d/%d pods ready, phases=%s, remaining=%s",
						params.Namespace, len(phases)-notReady, len(phases), stdout,
						time.Until(deadline).Truncate(time.Second))
				} else if stdout == "" {
					log.Debugf("TOOL", "wait_for_pods_ready: namespace=%s — no pods found yet, remaining=%s",
						params.Namespace, time.Until(deadline).Truncate(time.Second))
				}

				select {
				case <-ctx.Done():
					log.Warnf("TOOL", "wait_for_pods_ready: namespace=%s context cancelled after %s (%d polls)",
						params.Namespace, time.Since(pollStart), polls)
					return &ToolResult{Error: ctx.Err()}, nil
				case <-time.After(10 * time.Second):
				}
			}

			// Get current pod status for error context
			args := []string{"get", "pods", "-n", params.Namespace}
			if params.Label != "" {
				args = append(args, "-l", params.Label)
			}
			stdout, _, _ := oc.Run(ctx, args...)
			log.Errorf("TOOL", "wait_for_pods_ready: TIMEOUT in namespace=%s after %s (%d polls):\n%s",
				params.Namespace, time.Since(pollStart), polls, stdout)
			return &ToolResult{Error: fmt.Errorf("timeout waiting for pods to be ready in %s:\n%s", params.Namespace, stdout)}, nil
		},
	})
}
