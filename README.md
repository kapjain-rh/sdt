# SDT - Spec-Driven Testing for OpenShift

SDT is an AI-powered test framework for OpenShift. Tests are written as Markdown specs in natural language. An LLM agent reads each spec, plans execution steps, and runs them autonomously against a live OpenShift cluster via MCP tools.

Instead of writing fragile imperative test scripts, you describe **what** to test in plain English. The AI agent figures out **how** to execute it.

## Why SDT?

- **Natural language test specs** -- Tests are readable Markdown files, not code. Anyone who understands the product can write, review, and maintain them.
- **Zero test code maintenance** -- No test framework bindings, no selector breakage, no API version chasing. The agent adapts to cluster state dynamically.
- **Autonomous execution** -- The LLM agent plans steps, calls OpenShift tools, interprets results, retries on transient failures, and validates outcomes -- all without human intervention.
- **Plan caching** -- Execution plans are cached by spec content hash. Re-runs skip planning and go straight to execution, saving time and tokens.
- **Multi-provider LLM support** -- Works with Claude (Anthropic API or Vertex AI) and Google Gemini out of the box.
- **Suite orchestration** -- Suite-level and group-level hooks let you share setup/teardown logic across tests without duplication.
- **CI-ready reporting** -- JUnit XML output, console reporting, and Kiwi TCMS integration for test case management and result tracking.
- **Fixture system** -- Parameterized YAML resource definitions with lifecycle instructions the agent interprets at runtime.

## How It Works

When you run `sdt run`, a multi-agent pipeline executes for each spec:

```
Spec (Markdown) --> Memory Agent --> Planner Agent --> Executor Agent --> Results
                    (check cache)   (create plan)     (run via tools)   (report)
```

1. **Memory Agent** checks the plan cache for a previously generated plan.
2. **Planner Agent** reads the spec and produces a structured execution plan with phases, steps, and tool mappings.
3. **Executor Agent** runs the plan in auto-pilot mode, calling MCP tools (`oc_get`, `oc_apply`, `wait_for_pods_ready`, `shell`, etc.) in a multi-turn loop until completion.
4. **Reviewer Agent** (optional) reviews spec quality or analyzes execution failures.
5. Results are reported to the console, JUnit XML, and/or Kiwi TCMS.

## Installation

### Prerequisites

- Go 1.21+
- `oc` CLI logged into an OpenShift cluster (`oc whoami` should succeed)
- An Anthropic API key **or** Google Cloud credentials for Vertex AI
- Docker (optional, for Kiwi TCMS)

### Build from source

```bash
git clone https://github.com/openshift/sdt.git
cd sdt
make build        # builds binary to bin/sdt
```

Or install directly to your `$GOPATH/bin`:

```bash
make install
```

### Verify installation

```bash
sdt --version
```

### Environment setup

Set one of the following credential pairs:

```bash
# Option A: Anthropic API key (direct)
export ANTHROPIC_API_KEY=sk-ant-...

# Option B: Vertex AI (Claude on Google Cloud)
export ANTHROPIC_VERTEX_PROJECT_ID=my-project
export CLOUD_ML_REGION=us-east5

# Option C: Gemini
export SDT_PROVIDER=gemini
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_REGION=us-central1
```

Optional settings:

| Variable | Default | Description |
|---|---|---|
| `SDT_PROVIDER` | `claude` | LLM provider: `claude` or `gemini` |
| `SDT_MODEL` | `claude-opus-4-6` / `gemini-2.5-pro` | Override model name |
| `SDT_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig |

## Quick Start

### 1. Write a test spec

Create a Markdown file describing your test in natural language:

```markdown
# Test: Verify Pod Runs Successfully

## Metadata
- Author: yourname
- Priority: Critical
- Labels: [Smoke]
- Timeout: 10m

## Setup
Create namespace `my-test-ns` and deploy an nginx pod.

## Steps
1. Get the nginx pod in namespace `my-test-ns` and verify its status is Running.
2. Execute `curl -s localhost` inside the nginx pod and verify it returns the default welcome page.
3. Check that the pod has no restarts.

## Verify
- The nginx pod is in Running phase with all containers ready.
- The curl output contains "Welcome to nginx".
- Pod restart count is 0.

## Cleanup
Delete namespace `my-test-ns`.
```

### 2. Validate the spec (no cluster or LLM needed)

```bash
sdt validate specs/myproduct/
```

### 3. Run the test

```bash
# Run a single spec
sdt run specs/myproduct/my-test.md

# Run all specs in a directory
sdt run specs/myproduct/

# Dry run (plan only, no execution)
sdt run --dry-run specs/myproduct/my-test.md

# With JUnit output for CI
sdt run --junit-dir results/ specs/myproduct/
```

### 4. Other commands

```bash
sdt list specs/myproduct/                # List all specs
sdt list --format json specs/myproduct/  # List as JSON
sdt review specs/myproduct/my-test.md    # AI review of spec quality
sdt cache status specs/myproduct/        # Check plan cache
sdt cache clear                          # Clear all cached plans
```

## Project Structure

```
sdt/
├── cmd/sdt/                  # CLI entry point and subcommands
│   ├── main.go               #   root command (cobra)
│   ├── run.go                #   `sdt run` -- execute specs
│   ├── list.go               #   `sdt list` -- list specs
│   ├── validate.go           #   `sdt validate` -- check spec structure
│   ├── review.go             #   `sdt review` -- AI spec review
│   ├── cache.go              #   `sdt cache` -- plan cache management
│   └── tcms.go               #   `sdt tcms` -- Kiwi TCMS integration
│
├── pkg/
│   ├── agent/                # LLM agent implementations
│   │   ├── planner.go        #   creates execution plans from specs
│   │   ├── executor.go       #   runs plans via agentic tool-use loop
│   │   ├── memory.go         #   plan cache lookup
│   │   ├── reviewer.go       #   spec quality review / failure analysis
│   │   ├── coding.go         #   YAML template generation
│   │   └── prompts.go        #   system prompts for all agents
│   │
│   ├── llm/                  # LLM client abstraction
│   │   ├── provider.go       #   Provider interface
│   │   ├── client.go         #   shared client logic (Chat, RunAgentLoop)
│   │   ├── claude.go         #   Anthropic Messages API provider
│   │   ├── gemini.go         #   Google Gemini provider
│   │   └── types.go          #   message/tool types
│   │
│   ├── tools/                # MCP tool registry
│   │   ├── registry.go       #   tool registration and dispatch
│   │   ├── oc.go             #   oc_run, oc_apply, oc_delete, oc_get, oc_patch, oc_exec, oc_logs
│   │   ├── resource.go       #   create_namespace, delete_namespace, wait_for_*
│   │   ├── operator.go       #   deploy_operator, process_template
│   │   ├── metrics.go        #   query_metric (PromQL via thanos-querier)
│   │   ├── shell.go          #   shell, read_file, write_file
│   │   └── constraints.go    #   tool input validation
│   │
│   ├── spec/                 # Spec parsing and loading
│   │   ├── parser.go         #   Markdown-to-TestSpec parser
│   │   ├── loader.go         #   directory/file loader with suite/group resolution
│   │   └── types.go          #   TestSpec, SuiteConfig types
│   │
│   ├── runner/               # Test orchestration
│   │   └── runner.go         #   RunSuite/RunSpec lifecycle
│   │
│   ├── cache/                # Plan and result caching
│   │   ├── store.go          #   content-hash-based plan cache
│   │   └── history.go        #   execution result history
│   │
│   ├── fixture/              # Fixture system
│   │   ├── types.go          #   Fixture type definitions
│   │   ├── loader.go         #   YAML fixture loader
│   │   └── manager.go        #   fixture lifecycle management
│   │
│   ├── template/             # Template processing
│   │   ├── processor.go      #   Go text/template and oc process
│   │   ├── registry.go       #   template discovery
│   │   └── types.go          #   template type definitions
│   │
│   ├── reporter/             # Result reporting
│   │   ├── types.go          #   Reporter interface, MultiReporter
│   │   ├── console.go        #   terminal output
│   │   └── junit.go          #   JUnit XML output
│   │
│   ├── tcms/                 # Kiwi TCMS integration
│   │   ├── client.go         #   JSON-RPC client
│   │   ├── sync.go           #   spec-to-TCMS sync
│   │   └── reporter.go       #   TCMS result reporter
│   │
│   ├── mcp/                  # MCP server
│   │   ├── server.go         #   MCP protocol server
│   │   └── tools.go          #   tool schema exposure
│   │
│   └── log/                  # Structured logging
│       └── log.go
│
├── specs/                    # Test spec suites
│   ├── examples/             #   example suite with sample specs
│   └── netobserv/            #   Network Observability test suite
│
├── fixtures/                 # Fixture definitions (YAML)
│   ├── examples/
│   └── netobserv/
│
├── templates/                # Kubernetes/OpenShift YAML templates
│   ├── common/               #   generic pod, deployment, service, configmap
│   ├── operators/            #   subscription, operatorgroup, namespace
│   ├── networking/           #   nginx client/server
│   └── netobserv/            #   FlowCollector, Kafka, Loki, etc.
│
├── docs/                     # Documentation
├── Makefile                  # Build, test, lint, format targets
└── .sdt/cache/               # Local plan/result cache (gitignored)
```

## Use Cases

### Regression testing for OpenShift operators

Write specs that deploy an operator, configure its CR, verify expected pods and resources come up, and clean up. Run them nightly against development clusters with JUnit reporting for CI dashboards.

### Network observability end-to-end testing

The included `specs/netobserv/` suite covers 30+ test scenarios: flow correctness, Kafka with TLS, eBPF filtering, metrics, alerts, connection tracking, multi-tenancy, SCTP/ICMP, UDN, virtual machines, and more.

### Exploratory validation during development

Run a single spec against a dev cluster to quickly validate that a feature works as expected. The dry-run mode (`--dry-run`) lets you review the execution plan before committing to a real run.

### Test case management

Sync specs to Kiwi TCMS so QE teams can track test coverage, link specs to test plans, and view execution results alongside manual test efforts.

### Cross-team test authoring

Product managers, QE engineers, and developers can all write test specs -- no Go or test-framework expertise required. The LLM agent handles the implementation details.

## Suite and Group Hooks

Organize related tests into suites with shared setup/teardown:

```
specs/myproduct/
  _suite.md                  # Suite-level hooks
  _group_with_loki.md        # Group hooks for tests with Group: with-loki
  _group_with_kafka.md       # Group hooks for tests with Group: with-kafka
  sanity.md                  # Test spec
  alerts.md                  # Test spec (Group: with-loki)
  kafka_tls.md               # Test spec (Group: with-kafka)
```

Execution order:

```
Suite Pre-Suite (once)
  Pre-Suite Validation
    Suite Pre-Test
      Suite Pre-Test Validation
        Group Pre-Test
          Group Pre-Test Validation
            Setup -> Steps -> Verify -> Cleanup
          Group Post-Test
        Suite Post-Test
Suite Post-Suite (once)
```

## Fixtures

Fixtures are parameterized resource definitions with lifecycle instructions:

```yaml
name: flowcollector-default
description: Default FlowCollector with Direct deployment model
template: templates/netobserv/flowcollector_v1beta2_template.yaml
parameters:
  Namespace: openshift-netobserv
  DeploymentModel: Direct
  LokiEnable: "true"
lifecycle:
  create: >
    Use process_template to render the template with all listed parameters.
    Write the rendered output to a temp file, then use oc_apply to apply it.
  ready: >
    Wait for FlowCollector status condition Ready=True on flowcollector/cluster.
  cleanup: >
    oc_delete flowcollector cluster
```

Reference fixtures in a spec's metadata:

```markdown
## Metadata
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
```

## Available Tools

The agent can call these MCP tools during execution:

| Category | Tools |
|---|---|
| **OpenShift CLI** | `oc_run`, `oc_apply`, `oc_delete`, `oc_get`, `oc_patch`, `oc_exec`, `oc_logs` |
| **Resources** | `create_namespace`, `delete_namespace`, `wait_for_condition`, `wait_for_pods_ready` |
| **Operators** | `deploy_operator`, `process_template` |
| **Metrics** | `query_metric` (PromQL via thanos-querier) |
| **Shell** | `shell`, `read_file`, `write_file` |

## Kiwi TCMS Integration

SDT integrates with [Kiwi TCMS](https://kiwitcms.org) for test case management:

```bash
# Start local Kiwi TCMS
docker compose -f docker-compose.kiwi.yml up -d

# Set credentials
export KIWI_TCMS_URL=https://localhost:8443
export KIWI_TCMS_USERNAME=admin
export KIWI_TCMS_PASSWORD=admin

# Sync specs as test cases
sdt tcms sync --product NetObserv specs/netobserv/

# Check linkage status
sdt tcms status specs/netobserv/

# Run with TCMS reporting
sdt run --tcms --tcms-product NetObserv specs/netobserv/

# Import test cases from TCMS
sdt tcms import --plan-id 1 --output specs/imported/
```

## Contributing

### Development setup

```bash
git clone https://github.com/openshift/sdt.git
cd sdt
make build
make test
```

### Build commands

| Command | Description |
|---|---|
| `make build` | Build binary to `bin/sdt` |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run all tests (verbose, no caching) |
| `make test-short` | Run tests in short mode |
| `make lint` | Run `go vet` |
| `make fmt` | Format code with `gofmt` |
| `make fmt-check` | Check formatting (CI-friendly) |

Run a single test:

```bash
go test ./pkg/cache/... -v -count=1 -run TestStorePlanCaching
```

### Writing test specs

1. Create a new `.md` file in the appropriate `specs/` directory.
2. Follow the spec format: `# Test:` title, `## Metadata`, `## Setup`, `## Steps`, `## Verify`, `## Cleanup`.
3. Validate with `sdt validate`.
4. Review with `sdt review` for AI-assisted quality feedback.
5. Run with `sdt run --dry-run` first, then `sdt run`.

### Adding MCP tools

1. Define the tool handler function in the appropriate file under `pkg/tools/`.
2. Register it in `pkg/tools/registry.go` via `RegisterAllTools()`.
3. Provide a JSON schema for the tool's input parameters.
4. Return a `*ToolResult` from the handler.

### Adding templates

Place parameterized YAML templates in the `templates/` directory, organized by component. Templates support Go `text/template` syntax and `oc process` parameter substitution.

### Adding fixtures

Create a YAML file in `fixtures/` with `name`, `description`, `template`, `parameters`, and `lifecycle` fields. The lifecycle instructions are natural language that the LLM agent follows at runtime.

### Code style

- Run `make fmt` before committing.
- Run `make lint` to catch issues.
- Run `make test` to ensure nothing is broken.

## License

See [LICENSE](LICENSE) for details.