package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ocClient struct {
	ocPath     string
	kubeconfig string
}

func newOCClient() (*ocClient, error) {
	ocPath, err := exec.LookPath("oc")
	if err != nil {
		return nil, fmt.Errorf("oc binary not found in PATH: %w", err)
	}
	return &ocClient{
		ocPath:     ocPath,
		kubeconfig: os.Getenv("KUBECONFIG"),
	}, nil
}

func (c *ocClient) run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, c.ocPath, args...)
	if c.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+c.kubeconfig)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func (c *ocClient) runWithStdin(ctx context.Context, stdinData string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, c.ocPath, args...)
	if c.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+c.kubeconfig)
	}
	cmd.Stdin = strings.NewReader(stdinData)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

var tools = []toolDef{
	{
		Name:        "oc_run",
		Description: "Run any oc command. Use only for operations not covered by other tools.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"args":{"type":"array","items":{"type":"string"},"description":"Arguments to pass to oc"}},"required":["args"]}`),
	},
	{
		Name:        "oc_apply",
		Description: "Apply a YAML resource file to a namespace.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"file":{"type":"string","description":"Path to the YAML file"},"namespace":{"type":"string","description":"Namespace (optional)"}},"required":["file"]}`),
	},
	{
		Name:        "oc_delete",
		Description: "Delete a Kubernetes resource by kind, name, and namespace.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Resource kind"},"name":{"type":"string","description":"Resource name"},"namespace":{"type":"string","description":"Namespace (optional)"},"wait":{"type":"boolean","description":"Wait for deletion (default: true)"}},"required":["kind","name"]}`),
	},
	{
		Name:        "oc_get",
		Description: "Get a Kubernetes resource. Supports jsonpath output.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Resource kind"},"name":{"type":"string","description":"Resource name (optional)"},"namespace":{"type":"string","description":"Namespace (optional)"},"output":{"type":"string","description":"Output format: json, yaml, wide, name, or jsonpath=<template>"}},"required":["kind"]}`),
	},
	{
		Name:        "oc_patch",
		Description: "Patch a Kubernetes resource.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Resource kind"},"name":{"type":"string","description":"Resource name"},"namespace":{"type":"string","description":"Namespace (optional)"},"patch_type":{"type":"string","description":"Patch type: merge, json, strategic (default: merge)"},"patch":{"type":"string","description":"JSON patch content"}},"required":["kind","name","patch"]}`),
	},
	{
		Name:        "oc_exec",
		Description: "Execute a command inside a running pod.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"pod":{"type":"string","description":"Pod name"},"namespace":{"type":"string","description":"Namespace"},"container":{"type":"string","description":"Container name (optional)"},"command":{"type":"array","items":{"type":"string"},"description":"Command and arguments"}},"required":["pod","namespace","command"]}`),
	},
	{
		Name:        "oc_logs",
		Description: "Get logs from a pod.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"pod":{"type":"string","description":"Pod name"},"namespace":{"type":"string","description":"Namespace"},"container":{"type":"string","description":"Container name (optional)"},"tail":{"type":"integer","description":"Lines from end (default: 100)"},"previous":{"type":"boolean","description":"Show previous container logs"}},"required":["pod","namespace"]}`),
	},
	{
		Name:        "create_namespace",
		Description: "Create a Kubernetes namespace. Idempotent.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Namespace name"},"labels":{"type":"object","additionalProperties":{"type":"string"},"description":"Labels (optional)"}},"required":["name"]}`),
	},
	{
		Name:        "delete_namespace",
		Description: "Delete a Kubernetes namespace and wait for termination.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Namespace name"}},"required":["name"]}`),
	},
	{
		Name:        "wait_for_condition",
		Description: "Wait until a resource reaches a condition or jsonpath value matches.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","description":"Resource kind"},"name":{"type":"string","description":"Resource name"},"namespace":{"type":"string","description":"Namespace (optional)"},"condition":{"type":"string","description":"Condition to wait for"},"jsonpath":{"type":"string","description":"JSONPath expression (alternative to condition)"},"expected":{"type":"string","description":"Expected value for jsonpath"},"timeout":{"type":"string","description":"Timeout (e.g., 300s, 10m). Default: 300s"}},"required":["kind","name"]}`),
	},
	{
		Name:        "wait_for_pods_ready",
		Description: "Wait until all pods in a namespace are Running/Succeeded.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"namespace":{"type":"string","description":"Namespace"},"label":{"type":"string","description":"Label selector (optional)"},"timeout":{"type":"string","description":"Timeout (e.g., 300s, 10m). Default: 300s"}},"required":["namespace"]}`),
	},
	{
		Name:        "deploy_operator",
		Description: "Deploy an OLM operator by creating Namespace, OperatorGroup, and Subscription.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Operator package name"},"namespace":{"type":"string","description":"Install namespace"},"channel":{"type":"string","description":"Channel (default: stable)"},"source":{"type":"string","description":"CatalogSource (default: redhat-operators)"},"source_namespace":{"type":"string","description":"CatalogSource namespace (default: openshift-marketplace)"},"install_plan_approval":{"type":"string","description":"Automatic or Manual (default: Automatic)"}},"required":["name","namespace"]}`),
	},
	{
		Name:        "process_template",
		Description: "Process an OpenShift template with parameters. Returns rendered YAML.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"template":{"type":"string","description":"Template file path"},"parameters":{"type":"object","additionalProperties":{"type":"string"},"description":"Template parameters"},"namespace":{"type":"string","description":"Namespace (optional)"}},"required":["template"]}`),
	},
	{
		Name:        "query_metric",
		Description: "Query a Prometheus metric from the OpenShift monitoring stack.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"PromQL query string"},"namespace":{"type":"string","description":"Monitoring namespace (default: openshift-monitoring)"}},"required":["query"]}`),
	},
}

func handleToolCall(ctx context.Context, oc *ocClient, name string, args json.RawMessage) (string, error) {
	switch name {
	case "oc_run":
		var p struct{ Args []string `json:"args"` }
		json.Unmarshal(args, &p)
		stdout, stderr, err := oc.run(ctx, p.Args...)
		out := stdout
		if stderr != "" {
			out += "\nSTDERR: " + stderr
		}
		if err != nil {
			return out, fmt.Errorf("oc command failed: %w", err)
		}
		return out, nil

	case "oc_apply":
		var p struct {
			File      string `json:"file"`
			Namespace string `json:"namespace"`
		}
		json.Unmarshal(args, &p)
		a := []string{"apply", "-f", p.File}
		if p.Namespace != "" {
			a = append(a, "-n", p.Namespace)
		}
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("oc apply failed: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "oc_delete":
		var p struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Wait      *bool  `json:"wait"`
		}
		json.Unmarshal(args, &p)
		a := []string{"delete", p.Kind, p.Name}
		if p.Namespace != "" {
			a = append(a, "-n", p.Namespace)
		}
		if p.Wait == nil || *p.Wait {
			a = append(a, "--wait=true")
		}
		a = append(a, "--ignore-not-found")
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("oc delete failed: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "oc_get":
		var p struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Output    string `json:"output"`
		}
		json.Unmarshal(args, &p)
		a := []string{"get", p.Kind}
		if p.Name != "" {
			a = append(a, p.Name)
		}
		if p.Namespace != "" {
			a = append(a, "-n", p.Namespace)
		}
		if p.Output != "" {
			a = append(a, "-o", p.Output)
		}
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("oc get failed: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "oc_patch":
		var p struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			PatchType string `json:"patch_type"`
			Patch     string `json:"patch"`
		}
		json.Unmarshal(args, &p)
		pt := p.PatchType
		if pt == "" {
			pt = "merge"
		}
		a := []string{"patch", p.Kind, p.Name, "--type", pt, "-p", p.Patch}
		if p.Namespace != "" {
			a = append(a, "-n", p.Namespace)
		}
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("oc patch failed: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "oc_exec":
		var p struct {
			Pod       string   `json:"pod"`
			Namespace string   `json:"namespace"`
			Container string   `json:"container"`
			Command   []string `json:"command"`
		}
		json.Unmarshal(args, &p)
		a := []string{"exec", p.Pod, "-n", p.Namespace}
		if p.Container != "" {
			a = append(a, "-c", p.Container)
		}
		a = append(a, "--")
		a = append(a, p.Command...)
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("oc exec failed: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "oc_logs":
		var p struct {
			Pod       string `json:"pod"`
			Namespace string `json:"namespace"`
			Container string `json:"container"`
			Tail      int    `json:"tail"`
			Previous  bool   `json:"previous"`
		}
		json.Unmarshal(args, &p)
		a := []string{"logs", p.Pod, "-n", p.Namespace}
		if p.Container != "" {
			a = append(a, "-c", p.Container)
		}
		tail := p.Tail
		if tail == 0 {
			tail = 100
		}
		a = append(a, fmt.Sprintf("--tail=%d", tail))
		if p.Previous {
			a = append(a, "--previous")
		}
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("oc logs failed: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "create_namespace":
		var p struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		}
		json.Unmarshal(args, &p)
		stdout, _, err := oc.run(ctx, "get", "namespace", p.Name, "--ignore-not-found")
		if err == nil && strings.Contains(stdout, p.Name) {
			return fmt.Sprintf("Namespace %s already exists", p.Name), nil
		}
		stdout, stderr, err := oc.run(ctx, "create", "namespace", p.Name)
		if err != nil {
			return stdout, fmt.Errorf("creating namespace: %w\n%s", err, stderr)
		}
		for k, v := range p.Labels {
			_, stderr, err := oc.run(ctx, "label", "namespace", p.Name, fmt.Sprintf("%s=%s", k, v), "--overwrite")
			if err != nil {
				return stdout, fmt.Errorf("labeling namespace: %w\n%s", err, stderr)
			}
		}
		return fmt.Sprintf("Namespace %s created", p.Name), nil

	case "delete_namespace":
		var p struct{ Name string `json:"name"` }
		json.Unmarshal(args, &p)
		stdout, stderr, err := oc.run(ctx, "delete", "namespace", p.Name, "--ignore-not-found", "--wait=true")
		if err != nil {
			return stdout, fmt.Errorf("deleting namespace: %w\n%s", err, stderr)
		}
		return fmt.Sprintf("Namespace %s deleted", p.Name), nil

	case "wait_for_condition":
		var p struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Condition string `json:"condition"`
			JSONPath  string `json:"jsonpath"`
			Expected  string `json:"expected"`
			Timeout   string `json:"timeout"`
		}
		json.Unmarshal(args, &p)
		timeout := p.Timeout
		if timeout == "" {
			timeout = "300s"
		}
		timeoutDur, err := time.ParseDuration(timeout)
		if err != nil {
			timeoutDur = 300 * time.Second
		}
		if p.Condition != "" {
			a := []string{"wait", fmt.Sprintf("%s/%s", p.Kind, p.Name),
				fmt.Sprintf("--for=condition=%s", p.Condition),
				fmt.Sprintf("--timeout=%s", timeoutDur)}
			if p.Namespace != "" {
				a = append(a, "-n", p.Namespace)
			}
			stdout, stderr, err := oc.run(ctx, a...)
			if err != nil {
				return stdout, fmt.Errorf("wait for condition failed: %w\n%s", err, stderr)
			}
			return stdout, nil
		}
		if p.JSONPath == "" {
			return "", fmt.Errorf("either condition or jsonpath must be specified")
		}
		deadline := time.Now().Add(timeoutDur)
		for time.Now().Before(deadline) {
			a := []string{"get", p.Kind, p.Name, "-o", fmt.Sprintf("jsonpath=%s", p.JSONPath)}
			if p.Namespace != "" {
				a = append(a, "-n", p.Namespace)
			}
			stdout, _, err := oc.run(ctx, a...)
			if err == nil && strings.Contains(stdout, p.Expected) {
				return fmt.Sprintf("Condition met: %s = %s", p.JSONPath, stdout), nil
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}
		return "", fmt.Errorf("timeout waiting for %s/%s %s=%s", p.Kind, p.Name, p.JSONPath, p.Expected)

	case "wait_for_pods_ready":
		var p struct {
			Namespace string `json:"namespace"`
			Label     string `json:"label"`
			Timeout   string `json:"timeout"`
		}
		json.Unmarshal(args, &p)
		timeout := p.Timeout
		if timeout == "" {
			timeout = "300s"
		}
		timeoutDur, err := time.ParseDuration(timeout)
		if err != nil {
			timeoutDur = 300 * time.Second
		}
		deadline := time.Now().Add(timeoutDur)
		for time.Now().Before(deadline) {
			a := []string{"get", "pods", "-n", p.Namespace, "-o", "jsonpath={.items[*].status.phase}"}
			if p.Label != "" {
				a = append(a, "-l", p.Label)
			}
			stdout, _, err := oc.run(ctx, a...)
			if err == nil && stdout != "" {
				phases := strings.Fields(stdout)
				allRunning := len(phases) > 0
				for _, ph := range phases {
					if ph != "Running" && ph != "Succeeded" {
						allRunning = false
						break
					}
				}
				if allRunning {
					return fmt.Sprintf("All %d pods are Ready in namespace %s", len(phases), p.Namespace), nil
				}
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}
		a := []string{"get", "pods", "-n", p.Namespace}
		if p.Label != "" {
			a = append(a, "-l", p.Label)
		}
		stdout, _, _ := oc.run(ctx, a...)
		return "", fmt.Errorf("timeout waiting for pods in %s:\n%s", p.Namespace, stdout)

	case "deploy_operator":
		var p struct {
			Name                string `json:"name"`
			Namespace           string `json:"namespace"`
			Channel             string `json:"channel"`
			Source              string `json:"source"`
			SourceNamespace     string `json:"source_namespace"`
			InstallPlanApproval string `json:"install_plan_approval"`
		}
		json.Unmarshal(args, &p)
		channel := p.Channel
		if channel == "" {
			channel = "stable"
		}
		source := p.Source
		if source == "" {
			source = "redhat-operators"
		}
		sourceNS := p.SourceNamespace
		if sourceNS == "" {
			sourceNS = "openshift-marketplace"
		}
		approval := p.InstallPlanApproval
		if approval == "" {
			approval = "Automatic"
		}
		nsYAML := fmt.Sprintf("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s", p.Namespace)
		_, stderr, err := oc.runWithStdin(ctx, nsYAML, "apply", "-f", "-")
		if err != nil {
			return "", fmt.Errorf("creating namespace: %w\n%s", err, stderr)
		}
		ogYAML := fmt.Sprintf("apiVersion: operators.coreos.com/v1\nkind: OperatorGroup\nmetadata:\n  name: %s-og\n  namespace: %s\nspec:\n  targetNamespaces:\n  - %s", p.Name, p.Namespace, p.Namespace)
		_, stderr, err = oc.runWithStdin(ctx, ogYAML, "apply", "-f", "-")
		if err != nil {
			return "", fmt.Errorf("creating OperatorGroup: %w\n%s", err, stderr)
		}
		subYAML := fmt.Sprintf("apiVersion: operators.coreos.com/v1alpha1\nkind: Subscription\nmetadata:\n  name: %s\n  namespace: %s\nspec:\n  channel: %s\n  name: %s\n  source: %s\n  sourceNamespace: %s\n  installPlanApproval: %s", p.Name, p.Namespace, channel, p.Name, source, sourceNS, approval)
		_, stderr, err = oc.runWithStdin(ctx, subYAML, "apply", "-f", "-")
		if err != nil {
			return "", fmt.Errorf("creating Subscription: %w\n%s", err, stderr)
		}
		return fmt.Sprintf("Operator %s deployment initiated in namespace %s (channel: %s, source: %s)", p.Name, p.Namespace, channel, source), nil

	case "process_template":
		var p struct {
			Template   string            `json:"template"`
			Parameters map[string]string `json:"parameters"`
			Namespace  string            `json:"namespace"`
		}
		json.Unmarshal(args, &p)
		a := []string{"process", "-f", p.Template, "--ignore-unknown-parameters=true"}
		for k, v := range p.Parameters {
			a = append(a, "-p", fmt.Sprintf("%s=%s", k, v))
		}
		if p.Namespace != "" {
			a = append(a, "-n", p.Namespace)
		}
		stdout, stderr, err := oc.run(ctx, a...)
		if err != nil {
			return stdout, fmt.Errorf("processing template: %w\n%s", err, stderr)
		}
		return stdout, nil

	case "query_metric":
		var p struct {
			Query     string `json:"query"`
			Namespace string `json:"namespace"`
		}
		json.Unmarshal(args, &p)
		ns := p.Namespace
		if ns == "" {
			ns = "openshift-monitoring"
		}
		routeOut, stderr, err := oc.run(ctx, "get", "route", "thanos-querier", "-n", ns, "-o", "jsonpath={.spec.host}")
		if err != nil {
			return "", fmt.Errorf("getting thanos-querier route: %w\n%s", err, stderr)
		}
		tokenOut, stderr, err := oc.run(ctx, "whoami", "-t")
		if err != nil {
			return "", fmt.Errorf("getting token: %w\n%s", err, stderr)
		}
		curlArgs := []string{"exec", "-n", ns, "deploy/prometheus-operator", "--",
			"curl", "-sk",
			"-H", fmt.Sprintf("Authorization: Bearer %s", strings.TrimSpace(tokenOut)),
			fmt.Sprintf("https://%s/api/v1/query?query=%s", routeOut, p.Query)}
		stdout, stderr, err := oc.run(ctx, curlArgs...)
		if err != nil {
			stdout, stderr, err = oc.run(ctx, "exec", "-n", ns, "deploy/prometheus-operator", "--",
				"curl", "-sk",
				fmt.Sprintf("https://thanos-querier.%s.svc:9091/api/v1/query?query=%s", ns, p.Query))
			if err != nil {
				return stdout, fmt.Errorf("querying metric: %w\n%s", err, stderr)
			}
		}
		return stdout, nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func main() {
	oc, err := newOCClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		if req.ID == nil {
			continue
		}

		var resp jsonRPCResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":   map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":     map[string]interface{}{"name": "openshift-mcp-server", "version": "1.0.0"},
			}

		case "tools/list":
			resp.Result = map[string]interface{}{"tools": tools}

		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			json.Unmarshal(req.Params, &params)

			output, err := handleToolCall(ctx, oc, params.Name, params.Arguments)
			if err != nil {
				text := output
				if text != "" {
					text += "\n"
				}
				text += "Error: " + err.Error()
				resp.Result = map[string]interface{}{
					"content": []textContent{{Type: "text", Text: text}},
					"isError": true,
				}
			} else {
				resp.Result = map[string]interface{}{
					"content": []textContent{{Type: "text", Text: output}},
				}
			}

		default:
			resp.Error = &jsonRPCError{Code: -32601, Message: "method not found"}
		}

		data, _ := json.Marshal(resp)
		fmt.Fprintf(os.Stdout, "%s\n", data)

		if req.Method == "initialize" {
			notify, _ := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "notifications/initialized",
			})
			fmt.Fprintf(os.Stdout, "%s\n", notify)
		}
	}
}
