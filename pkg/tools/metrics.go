package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openshift/sdt/pkg/log"
)

// RegisterMetricsTools registers Prometheus metric query tools.
func RegisterMetricsTools(registry *Registry, oc *OCClient) {
	registry.Register(&Tool{
		Name:        "query_metric",
		Description: "Query a Prometheus metric from the OpenShift monitoring stack. Returns the current value of the metric. Useful for verifying that metrics are being collected.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "PromQL query string (e.g., 'netobserv_flows_total', 'sum(rate(http_requests_total[5m]))')"
				},
				"namespace": {
					"type": "string",
					"description": "Namespace of the monitoring route (default: openshift-monitoring)"
				}
			},
			"required": ["query"]
		}`),
		Handler: func(ctx context.Context, input json.RawMessage) (*ToolResult, error) {
			var params struct {
				Query     string `json:"query"`
				Namespace string `json:"namespace"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return nil, fmt.Errorf("parsing input: %w", err)
			}

			ns := params.Namespace
			if ns == "" {
				ns = "openshift-monitoring"
			}

			log.Infof("TOOL", "query_metric: %s namespace=%s", params.Query, ns)

			// Get the thanos-querier route
			routeOut, stderr, err := oc.Run(ctx, "get", "route", "thanos-querier", "-n", ns, "-o", "jsonpath={.spec.host}")
			if err != nil {
				return &ToolResult{Error: fmt.Errorf("getting thanos-querier route: %w\n%s", err, stderr)}, nil
			}

			// Get a token for authentication
			tokenOut, stderr, err := oc.Run(ctx, "whoami", "-t")
			if err != nil {
				return &ToolResult{Error: fmt.Errorf("getting token: %w\n%s", err, stderr)}, nil
			}

			// Query Prometheus via the route
			curlArgs := []string{
				"exec", "-n", ns,
				"deploy/prometheus-operator", "--",
				"curl", "-sk",
				"-H", fmt.Sprintf("Authorization: Bearer %s", strings.TrimSpace(tokenOut)),
				fmt.Sprintf("https://%s/api/v1/query?query=%s", routeOut, params.Query),
			}

			stdout, stderr, err := oc.Run(ctx, curlArgs...)
			if err != nil {
				// Fallback: try using oc with direct API
				stdout, stderr, err = oc.Run(ctx, "exec", "-n", ns,
					"deploy/prometheus-operator", "--",
					"curl", "-sk",
					fmt.Sprintf("https://thanos-querier.%s.svc:9091/api/v1/query?query=%s", ns, params.Query),
				)
				if err != nil {
					return &ToolResult{Output: stdout, Error: fmt.Errorf("querying metric: %w\n%s", err, stderr)}, nil
				}
			}

			return &ToolResult{Output: stdout}, nil
		},
	})
}
