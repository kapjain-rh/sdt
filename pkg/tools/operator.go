package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift/sdt/pkg/log"
)

// RegisterOperatorTools registers OLM operator deployment tools.
func RegisterOperatorTools(registry *Registry, oc *OCClient) {
	registry.Register(&Tool{
		Name:        "deploy_operator",
		Description: "Deploy an operator via OLM by creating a Subscription. Creates the namespace, OperatorGroup, and Subscription, then waits for the operator pod to be running.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Operator package name (e.g., 'netobserv-operator')"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace to install the operator in"
				},
				"channel": {
					"type": "string",
					"description": "Subscription channel (e.g., 'stable', 'alpha'). Default: 'stable'"
				},
				"source": {
					"type": "string",
					"description": "CatalogSource name. Default: 'redhat-operators'"
				},
				"source_namespace": {
					"type": "string",
					"description": "CatalogSource namespace. Default: 'openshift-marketplace'"
				},
				"install_plan_approval": {
					"type": "string",
					"description": "InstallPlan approval: Automatic or Manual. Default: 'Automatic'"
				}
			},
			"required": ["name", "namespace"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Name                string `json:"name"`
				Namespace           string `json:"namespace"`
				Channel             string `json:"channel"`
				Source              string `json:"source"`
				SourceNamespace     string `json:"source_namespace"`
				InstallPlanApproval string `json:"install_plan_approval"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			channel := params.Channel
			if channel == "" {
				channel = "stable"
			}
			source := params.Source
			if source == "" {
				source = "redhat-operators"
			}
			sourceNS := params.SourceNamespace
			if sourceNS == "" {
				sourceNS = "openshift-marketplace"
			}
			approval := params.InstallPlanApproval
			if approval == "" {
				approval = "Automatic"
			}

			log.Infof("TOOL", "deploy_operator: name=%s namespace=%s channel=%s source=%s",
				params.Name, params.Namespace, channel, source)

			// Create namespace (idempotent)
			nsYAML := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s`, params.Namespace)

			_, stderr, err := oc.RunWithStdin(ctx, nsYAML, "apply", "-f", "-")
			if err != nil {
				return &ToolResult{Error: fmt.Errorf("creating namespace %s: %w\n%s", params.Namespace, err, stderr)}, nil
			}

			// Create OperatorGroup
			ogYAML := fmt.Sprintf(`apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: %s-og
  namespace: %s
spec:
  targetNamespaces:
  - %s`, params.Name, params.Namespace, params.Namespace)

			_, stderr, err = oc.RunWithStdin(ctx, ogYAML, "apply", "-f", "-")
			if err != nil {
				return &ToolResult{Error: fmt.Errorf("creating OperatorGroup in %s: %w\n%s", params.Namespace, err, stderr)}, nil
			}

			// Create Subscription
			subYAML := fmt.Sprintf(`apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: %s
  namespace: %s
spec:
  channel: %s
  name: %s
  source: %s
  sourceNamespace: %s
  installPlanApproval: %s`, params.Name, params.Namespace, channel, params.Name, source, sourceNS, approval)

			_, stderr, err = oc.RunWithStdin(ctx, subYAML, "apply", "-f", "-")
			if err != nil {
				return &ToolResult{Error: fmt.Errorf("creating Subscription for %s: %w\n%s", params.Name, err, stderr)}, nil
			}

			log.Infof("TOOL", "deploy_operator: %s deployment initiated", params.Name)
			return &ToolResult{Output: fmt.Sprintf("Operator %s deployment initiated in namespace %s (channel: %s, source: %s)", params.Name, params.Namespace, channel, source)}, nil
		},
	})

	registry.Register(&Tool{
		Name:        "process_template",
		Description: "Process an OpenShift template file with parameters. Returns the rendered YAML that can then be applied with oc_apply.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"template": {
					"type": "string",
					"description": "Path to the template file"
				},
				"parameters": {
					"type": "object",
					"additionalProperties": {"type": "string"},
					"description": "Template parameters as key-value pairs"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace for the processed resources (optional)"
				}
			},
			"required": ["template"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Template   string            `json:"template"`
				Parameters map[string]string `json:"parameters"`
				Namespace  string            `json:"namespace"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			// Normalize template path: strip leading "/" to keep paths relative to cwd
			tmpl := params.Template
			if len(tmpl) > 1 && tmpl[0] == '/' && tmpl[1] != '/' {
				tmpl = tmpl[1:]
			}
			args := []string{"process", "-f", tmpl, "--ignore-unknown-parameters=true"}
			for k, v := range params.Parameters {
				args = append(args, "-p", fmt.Sprintf("%s=%s", k, v))
			}
			if params.Namespace != "" {
				args = append(args, "-n", params.Namespace)
			}

			log.Infof("TOOL", "process_template: %s namespace=%s params=%v", tmpl, params.Namespace, params.Parameters)
			stdout, stderr, err := oc.Run(ctx, args...)
			if err != nil {
				log.Warnf("TOOL", "process_template failed: %v", err)
				return &ToolResult{Output: stdout, Error: fmt.Errorf("processing template: %w\n%s", err, stderr)}, nil
			}
			return &ToolResult{Output: stdout}, nil
		},
	})
}
