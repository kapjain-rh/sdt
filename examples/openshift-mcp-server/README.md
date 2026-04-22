# OpenShift MCP Server

A standalone MCP (Model Context Protocol) server that exposes OpenShift/Kubernetes tools for use with SDT.

## Prerequisites

- `oc` CLI binary in PATH
- Valid `KUBECONFIG` or active `oc login` session

## Build

```bash
go build -o openshift-mcp-server .
```

## Tools Provided

| Tool | Description |
|---|---|
| `oc_run` | Run any oc command (for ad-hoc operations) |
| `oc_apply` | Apply YAML resources |
| `oc_delete` | Delete resources |
| `oc_get` | Get resources (supports jsonpath) |
| `oc_patch` | Patch resources |
| `oc_exec` | Execute commands in pods |
| `oc_logs` | Get pod logs |
| `create_namespace` | Create namespace (idempotent) |
| `delete_namespace` | Delete namespace |
| `wait_for_condition` | Wait for resource condition or jsonpath value |
| `wait_for_pods_ready` | Wait for all pods to be Running |
| `deploy_operator` | Deploy OLM operator (Subscription + OperatorGroup) |
| `process_template` | Process OpenShift templates |
| `query_metric` | Query Prometheus metrics |

## Configure in SDT

Add to your project's `.sdt.yaml`:

```yaml
project: my-openshift-project
description: OpenShift cluster
context: |
  Use dedicated oc tools instead of shell commands.
  Default CatalogSource is redhat-operators in openshift-marketplace.

mcpServers:
  openshift:
    command: ./openshift-mcp-server
    env:
      KUBECONFIG: /path/to/kubeconfig
```

Then run tests:

```bash
sdt run sdt/specs/
```

SDT will automatically discover all 14 tools from this server at startup.
