# SDT — Spec-Driven Testing

SDT is a product-agnostic, AI-powered test framework. Tests are written as Markdown specs in natural language. An LLM agent reads each spec, plans execution steps, and runs them autonomously against any target system via MCP tools.

Instead of writing fragile imperative test scripts, you describe **what** to test in plain English. The AI agent figures out **how** to execute it.

## Why SDT?

- **Natural language test specs** — Tests are readable Markdown files, not code. Anyone who understands the product can write, review, and maintain them.
- **Product-agnostic** — Works with any system: web apps, Kubernetes clusters, APIs, databases, CLI tools. Bring your own tools via MCP servers or YAML definitions.
- **Zero test code maintenance** — No test framework bindings, no selector breakage, no API version chasing. The agent adapts dynamically.
- **Autonomous execution** — The LLM agent plans steps, calls tools, interprets results, retries on transient failures, and validates outcomes — all without human intervention.
- **Plan caching** — Execution plans are cached by spec content hash. Re-runs skip planning and go straight to execution, saving time and tokens.
- **Multi-provider LLM support** — Works with Claude (Anthropic API or Vertex AI) and Google Gemini out of the box.
- **Tool lifecycle** — Create, test, and approve tools interactively before using them in test runs.
- **CI-ready reporting** — JUnit XML output, console reporting, and Kiwi TCMS integration for test case management.

## How It Works

```
Spec (Markdown) --> Memory Agent --> Planner Agent --> Executor Agent --> Results
                    (check cache)   (create plan)     (run via tools)   (report)
```

1. **Memory Agent** checks the plan cache for a previously generated plan.
2. **Planner Agent** reads the spec and produces a structured execution plan with phases, steps, and tool mappings.
3. **Executor Agent** runs the plan in auto-pilot mode, calling tools in a multi-turn loop until completion.
4. **Reviewer Agent** (optional) reviews spec quality or analyzes execution failures.
5. Results are reported to the console, JUnit XML, and/or Kiwi TCMS.

## Installation

### Prerequisites

- Go 1.21+
- An Anthropic API key **or** Google Cloud credentials for Vertex AI

### Build from source

```bash
git clone https://github.com/sdt-project/sdt.git
cd sdt
make build        # builds binary to bin/sdt
```

Or install directly:

```bash
make install      # installs to $GOPATH/bin
```

### Environment setup

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

| Variable | Default | Description |
|---|---|---|
| `SDT_PROVIDER` | `claude` | LLM provider: `claude` or `gemini` |
| `SDT_MODEL` | `claude-opus-4-6` / `gemini-2.5-pro` | Override model name |
| `SDT_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Quick Start

### 1. Set up SDT in your project

```bash
cd my-project
sdt setup myapp
```

This creates:

```
my-project/
  .sdt.yaml                    # SDT configuration
  sdt/
    specs/
      _suite.md                # Suite-level hooks
      smoke-test.md            # Sample test spec
    fixtures/
      sample.yaml              # Sample fixture
    tools/                     # Custom tool definitions (YAML)
    mcp/
      README.md                # Guide for adding MCP servers
```

### 2. Add project-specific tools

```bash
# Create a tool via the default MCP server (YAML definition)
sdt tools add check_health \
  --description "Check application health" \
  --command "curl -sf {{.endpoint}}/health"

# Edit the generated YAML to refine parameters
vi sdt/tools/check_health.yaml

# Test the tool
sdt tools test check_health --input '{"endpoint": "http://localhost:8080"}'

# Approve it for use in test runs
sdt tools approve check_health

# List all available tools
sdt tools list
```

```
NAME            SOURCE          STATUS    DESCRIPTION
----            ------          ------    -----------
shell           core            -         Execute a local shell command
read_file       core            -         Read the contents of a local file
check_health    default         approved  Check application health
oc_get          mcp:openshift   -         Get OpenShift resources
```

### 3. Create and manage test specs

```bash
# Scaffold a draft spec
sdt specs add login-flow --author john --priority High --timeout 15m

# Edit the spec — add your test steps
vi sdt/specs/login-flow.md

# Validate structure (no LLM needed)
sdt validate sdt/specs/login-flow.md

# AI quality review
sdt review sdt/specs/login-flow.md

# Dry run — plan only, verify it works
sdt run --dry-run sdt/specs/login-flow.md

# Approve for production runs and TCMS sync
sdt specs approve sdt/specs/login-flow.md

# List all specs with status
sdt specs list
```

```
NAME                STATUS    PRIORITY  GROUP  CASEID  FILE
----                ------    --------  -----  ------  ----
Api Crud            draft     Critical  api    -       sdt/specs/api-crud.md
Login Flow          approved  High      -      -       sdt/specs/login-flow.md
Smoke Test          approved  high      smoke  -       sdt/specs/smoke-test.md

Total: 3 specs (2 approved, 1 draft)
```

A scaffolded spec looks like:

```markdown
# Test: Login Flow

## Metadata
- Author: john
- Priority: High
- Status: draft
- Timeout: 15m
- Labels: []

## Setup
1. TODO: Describe pre-test setup

## Steps
1. TODO: Describe the first test action
2. TODO: Describe the second test action

## Verify
1. TODO: Describe what to verify after steps complete

## Cleanup
1. TODO: Describe cleanup actions
```

**Spec lifecycle:**

```
sdt specs add        → creates spec (status: draft)
sdt validate         → check structure (no LLM)
sdt review           → AI quality review
sdt specs run        → run with section control (drafts included)
sdt specs approve    → status → approved
                        ↓
sdt run              → only approved specs run (--include-drafts to override)
sdt tcms sync        → only approved specs sync to TCMS
```

### 4. Run tests

**During development** — use `sdt specs run` with section control:

```bash
# Run only setup (verify your setup works)
sdt specs run sdt/specs/login-flow.md --only setup

# Run steps and verify, skip cleanup
sdt specs run sdt/specs/login-flow.md --only steps,verify

# Skip cleanup during debugging
sdt specs run sdt/specs/login-flow.md --skip cleanup

# Dry run — plan only
sdt specs run sdt/specs/login-flow.md --dry-run

# Run just cleanup (teardown from a previous run)
sdt specs run sdt/specs/login-flow.md --only cleanup
```

**In CI/production** — use `sdt run` (only approved specs):

```bash
# Run all approved specs
sdt run sdt/specs/

# Include draft specs
sdt run --include-drafts sdt/specs/

# Run a single spec
sdt run sdt/specs/login-flow.md

# Dry run (plan only, no execution)
sdt run --dry-run sdt/specs/login-flow.md

# With JUnit output for CI
sdt run --junit-dir results/ sdt/specs/
```

### 5. Organize into suites

```bash
# Create a test suite
sdt suite add regression --description "Full regression tests"
sdt suite add api-tests

# List all suites
sdt suite list
```

```
SUITE                  SPECS  APPROVED  DRAFT  GROUPS  PATH
-----                  -----  --------  -----  ------  ----
myapp Test Suite       3      2         1      0       sdt/specs
Api Tests Test Suite   2      2         0      0       sdt/specs/api-tests
Regression Test Suite  5      4         1      0       sdt/specs/regression
```

```bash
# Run a full suite
sdt suite run sdt/specs/regression

# Run suite with section control
sdt suite run sdt/specs/regression --only pre-suite,setup,steps,verify

# Skip cleanup and post-suite
sdt suite run sdt/specs/regression --skip cleanup,post-suite

# Run suite with JUnit and TCMS
sdt suite run sdt/specs/regression --junit-dir results/ --tcms --tcms-product MyApp
```

## Tools

SDT tools come from three sources:

### Core tools (built-in)

Always available, no configuration needed:

| Tool | Description |
|---|---|
| `shell` | Execute shell commands |
| `read_file` / `write_file` | File I/O |
| `python` / `node` / `go_run` | Language runtime execution |
| `npm` / `npx` / `cypress` | Node.js ecosystem tools |
| `check_runtimes` | Check available runtimes |

### Default MCP server (YAML tool definitions)

Simple command-based tools defined as YAML files in `sdt/tools/`. Managed via `sdt tools add/test/approve`.

```yaml
# sdt/tools/oc_get.yaml
name: oc_get
description: Get OpenShift resources in JSON format
category: openshift
status: approved
input:
  resource:
    type: string
    description: "Resource type (e.g., pods, deployments)"
    required: true
  namespace:
    type: string
    description: Target namespace
command: "oc get {{.resource}} {{if .namespace}}-n {{.namespace}}{{end}} -o json"
```

**Tool lifecycle:**

```bash
sdt tools add <name>           # Create draft tool
sdt tools test <name> --input  # Test with sample input
sdt tools approve <name>       # Promote to approved
sdt tools list                 # List all tools with status
```

Only approved tools are loaded during `sdt run`. Draft tools are skipped.

### Third-party MCP servers

External MCP servers for complex tools that need full code logic. Written in any language (Go, Python, TypeScript, bash) and configured in `.sdt.yaml`:

```yaml
mcpServers:
  openshift:
    command: ./sdt/mcp/openshift-server
  database:
    command: python
    args: [-m, db_mcp_server]
    env:
      DB_HOST: localhost
      DB_PORT: "5432"
```

SDT connects to each server at startup via JSON-RPC 2.0 over stdio, discovers tools via `tools/list`, and registers them in the tool registry.

Example Python MCP server:

```python
from mcp.server.stdio import stdio_server
from mcp.server import Server
from mcp import types
import subprocess

server = Server("openshift-tools")

@server.list_tools()
async def list_tools():
    return [
        types.Tool(
            name="oc_get",
            description="Get OpenShift resources",
            inputSchema={
                "type": "object",
                "properties": {
                    "resource": {"type": "string", "description": "Resource type"},
                    "namespace": {"type": "string", "description": "Namespace"}
                },
                "required": ["resource"]
            }
        )
    ]

@server.call_tool()
async def call_tool(name, arguments):
    if name == "oc_get":
        cmd = ["oc", "get", arguments["resource"], "-o", "json"]
        if arguments.get("namespace"):
            cmd += ["-n", arguments["namespace"]]
        result = subprocess.run(cmd, capture_output=True, text=True)
        return [types.TextContent(type="text", text=result.stdout or result.stderr)]

async def main():
    async with stdio_server() as (read, write):
        await server.run(read, write)

if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
```

## Use Cases

### Web application E2E testing

```bash
sdt setup webapp
sdt tools add check_health --command "curl -sf {{.url}}/health"
sdt tools add check_login --command "curl -sf -X POST -d '{\"user\":\"{{.user}}\",\"pass\":\"{{.pass}}\"}' {{.url}}/login"
sdt tools approve check_health
sdt tools approve check_login
```

Write specs that verify login flows, API endpoints, page loads, and error handling. The LLM agent orchestrates curl, health checks, and assertions autonomously.

### Kubernetes / OpenShift cluster testing

Configure an MCP server exposing `oc` / `kubectl` tools, then write specs that deploy operators, verify pod health, check metrics, and clean up resources. Run nightly with JUnit reporting for CI dashboards. See `examples/openshift-mcp-server/` for a complete Go-based MCP server with 14 tools.

```yaml
# .sdt.yaml
mcpServers:
  openshift:
    command: ./openshift-mcp-server
    env:
      KUBECONFIG: /path/to/kubeconfig
```

### Database validation

```bash
sdt tools add db_query --command "psql -h {{.host}} -U {{.user}} -d {{.database}} -c '{{.query}}'"
sdt tools approve db_query
```

Write specs that verify schema migrations, data integrity, query performance, and replication status.

### API contract testing

```bash
sdt tools add api_call --command "curl -sf -X {{.method}} -H 'Content-Type: application/json' -d '{{.body}}' {{.url}}"
sdt tools approve api_call
```

Write specs that verify API contracts: correct status codes, response schemas, error handling, rate limiting, and authentication flows.

### CI/CD pipeline validation

Write specs that verify deployment pipelines: build succeeds, tests pass, artifacts are published, staging deployment is healthy, smoke tests pass before production rollout.

### Infrastructure testing

Add tools for Terraform, Ansible, or cloud CLIs. Write specs that verify infrastructure provisioning, configuration drift detection, and disaster recovery procedures.

## Configuration

### .sdt.yaml

```yaml
project: myapp                    # Project name
description: "My application"     # System description for LLM context
specsDir: sdt/specs               # Test spec directory
fixturesDir: sdt/fixtures         # Fixture definitions
toolsDir: sdt/tools               # Custom tool definitions (YAML)
shellTimeout: 120s                # Shell command timeout (default: 60s)

# Extra context appended to LLM system prompts
context: |
  Use dedicated tools instead of shell commands where available.

# Global constraints — block shell commands that have dedicated tools
constraints:
  - block_shell: "curl"
    redirect: check_health

mcpServers:                       # MCP tool servers
  openshift:
    command: ./openshift-mcp-server
    env:
      KUBECONFIG: /path/to/kubeconfig
    constraints:                  # Per-server constraints
      - block_shell: "oc get"
        redirect: oc_get
      - block_shell: "oc apply"
        redirect: oc_apply
      - block_tool: oc_run        # Redirect within MCP tools
        match: wait
        redirect: wait_for_condition
```

### CLI flags

```bash
sdt run <spec-path>
  --timeout 30m                   # Default timeout per spec
  --junit-dir results/            # JUnit XML output directory
  --no-cache                      # Force re-planning
  --dry-run                       # Plan only, no execution
  --fixtures-dir fixtures/        # Override fixtures directory
  --context "cluster is on AWS"   # Extra LLM context
  --skip-cleanup                  # Skip cleanup phases
  --skip-phases pre-suite,cleanup # Skip specific phases
  --only-phases cleanup           # Run only these phases
  --tcms                          # Report to Kiwi TCMS
  --tcms-product NetObserv        # TCMS product name
  --tcms-plan-id 42               # Run specs matching a TCMS plan
```

## Suite and Group Hooks

Organize related tests into suites with shared setup/teardown:

```
sdt/specs/
  _suite.md                       # Suite-level hooks (runs once)
  _group_database.md              # Group hooks for tests with Group: database
  _group_api.md                   # Group hooks for tests with Group: api
  login-test.md                   # Test spec
  user-crud.md                    # Test spec (Group: database)
  search-api.md                   # Test spec (Group: api)
```

Execution order:

```
Suite Pre-Suite → Pre-Suite Validation
  ├── Suite Pre-Test → Pre-Test Validation
  │     ├── Group Pre-Test → Group Pre-Test Validation
  │     │     └── Setup → Steps → Verify → Cleanup
  │     │   Group Post-Test
  │     └── Suite Post-Test
Suite Post-Suite
```

Validation sections (`## Pre-Suite Validation`, `## Pre-Test Validation`) define conditions that must be true after hooks run. Hook errors are tolerated, but validation failures abort execution.

## Fixtures

Parameterized resource definitions with lifecycle instructions the LLM agent interprets at runtime.

**Fixture lifecycle:**

```
sdt fixtures add        → creates fixture (status: draft)
sdt fixtures validate   → check all fixtures parse correctly
sdt fixtures approve    → status → approved
                           ↓
sdt run                 → only approved fixtures resolved (--include-drafts to override)
```

```bash
# Create a draft fixture
sdt fixtures add test-database --description "PostgreSQL test database"

# Edit the generated YAML
vi sdt/fixtures/test-database.yaml

# Validate all fixtures
sdt fixtures validate

# Approve for use in test runs
sdt fixtures approve test-database

# List all fixtures
sdt fixtures list
```

```
NAME           STATUS    TEMPLATE  PARAMS  DESCRIPTION
----           ------    --------  ------  -----------
test-database  approved  -         2       PostgreSQL test database

Total: 1 fixtures (1 approved, 0 draft)
```

A fixture definition looks like:

```yaml
name: test-database
description: PostgreSQL test database
status: approved
parameters:
  name: testdb
  port: "5432"
lifecycle:
  create: "Create a PostgreSQL database named '${name}' on port ${port}"
  ready: "Verify the database '${name}' accepts connections"
  cleanup: "Drop the database '${name}' and remove all data"
```

Reference in specs:

```markdown
## Metadata
- Fixtures: [test-database]

## Setup
Deploy the test database using fixture `test-database`.
```

## CLI Commands

| Command | Description |
|---|---|
| **Project setup** | |
| `sdt setup <name>` | Set up SDT in current project directory |
| **Spec lifecycle** | |
| `sdt specs add <name>` | Create a draft test spec |
| `sdt specs run <spec>` | Run with section control (`--only`, `--skip`) |
| `sdt specs approve <spec>` | Approve a draft spec for runs and TCMS |
| `sdt specs list [dir]` | List all specs with lifecycle status |
| `sdt validate <specs>` | Validate spec structure (no LLM needed) |
| `sdt review <spec>` | AI review of spec quality |
| **Suite management** | |
| `sdt suite add <name>` | Create a new test suite (directory + hooks) |
| `sdt suite run <dir>` | Run a suite with section control |
| `sdt suite list` | List all suites with spec counts |
| **Fixture lifecycle** | |
| `sdt fixtures add <name>` | Create a draft fixture definition |
| `sdt fixtures validate` | Validate all fixture definitions |
| `sdt fixtures approve <name>` | Approve a draft fixture |
| `sdt fixtures list` | List all fixtures with status |
| **Tool lifecycle** | |
| `sdt tools add <name>` | Create a draft tool definition |
| `sdt tools test <name>` | Test a tool with sample input |
| `sdt tools approve <name>` | Approve a draft tool |
| `sdt tools list` | List all tools (core + default + MCP) |
| **Execution** | |
| `sdt run <specs>` | Run approved specs (`--include-drafts` to include all) |
| `sdt run --dry-run <specs>` | Plan only, no execution |
| **Cache** | |
| `sdt cache status` | Check plan cache |
| `sdt cache clear` | Clear cached plans |
| **TCMS** | |
| `sdt tcms sync` | Sync approved specs to Kiwi TCMS |
| `sdt tcms status` | Check TCMS linkage |
| `sdt tcms import` | Import test cases from TCMS |

## Web UI (SDT-TCMS)

SDT includes a web-based test management UI for browsing specs, fixtures, groups, suites, tools, and cached execution plans/results.

### Bundled UI (recommended)

The UI can be bundled directly into the `sdt` binary. A single command serves both the API and the web interface:

```bash
# From within your project directory
sdt serve --ui

# Or specify a project directory and port
sdt serve --ui --project-dir /path/to/my-project --port 8090
```

Open `http://localhost:8090` in your browser. The `--ui` flag serves the bundled frontend alongside the API — no separate frontend process needed.

**Building the bundled UI from source:**

```bash
# Build the frontend static export and embed it into the Go binary
make build-all

# Or step by step:
make build-ui    # exports frontend to pkg/api/ui/dist/
make build       # compiles Go binary with embedded UI
```

This requires Node.js 18+. The frontend source lives in `ui/` within this repo.

### Development mode

For frontend development, run the API and frontend dev server separately:

**1. Start the API server:**

```bash
sdt serve --project-dir /path/to/my-project --port 8090
```

**2. Start the frontend dev server:**

```bash
cd ui
npm install
npm run dev
```

The frontend runs on `http://localhost:3000` and proxies API requests to the backend on port 8090. To use a different backend port, set `SDT_API_PORT`:

```bash
SDT_API_PORT=9090 npm run dev
```

### Configuring the backend URL

The UI sidebar includes a **Backend** field where you can point the frontend to a different API server (e.g., `http://localhost:9090/api`). This is saved in your browser's localStorage and persists across sessions. Leave it blank to use the default (proxy in dev mode, same origin when bundled).

### What the UI shows

| Page | Description |
|---|---|
| **Dashboard** | Overview stats — total cases, plans, runs |
| **Suite** | Suite-level hooks (`_suite.md`) — pre-suite, pre-test, post-test, post-suite |
| **Groups** | Group hooks (`_group_*.md`) with associated specs |
| **Test Cases** | IDE-style spec editor with metadata, setup/steps/verify/cleanup sections. Includes a **Cache** tab showing the LLM-generated execution plan and past run results for each spec |
| **Fixtures** | YAML fixture editor with templates, parameters, and lifecycle (create/ready/cleanup) |
| **Test Plans** | Organize cases into plans |
| **Test Runs** | Execute plans, track results with interactive execution mode |
| **Tools** | Manage YAML tool definitions (draft → test → approve) |
| **MCP Servers** | Connect to MCP servers, discover and test tools |

### Cache visualization

When viewing a test case, the **Cache** tab shows:

- **Execution Plan** — the LLM-generated plan with phases, steps, tool mappings, and parameters. This is the plan the agent follows during execution — cached by spec content hash so the LLM is only called once per spec version.
- **Results** — past execution results with per-phase and per-step status (PASSED/FAILED/SKIPPED), error messages, duration, and tool output. Multiple runs are shown with a selector.

## Kiwi TCMS Integration

SDT integrates with [Kiwi TCMS](https://kiwitcms.org) for test case management:

```bash
# Set credentials
export KIWI_TCMS_URL=https://localhost:8443
export KIWI_TCMS_USERNAME=admin
export KIWI_TCMS_PASSWORD=admin

# Sync specs as test cases
sdt tcms sync --product MyApp sdt/specs/

# Run with TCMS reporting
sdt run --tcms --tcms-product MyApp sdt/specs/

# Import test cases from TCMS plan
sdt tcms import --plan-id 1 --output sdt/specs/imported/
```

## Contributing

```bash
git clone https://github.com/sdt-project/sdt.git
cd sdt
make build
make test
```

| Command | Description |
|---|---|
| `make build` | Build binary to `bin/sdt` |
| `make build-ui` | Build the frontend static export into `pkg/api/ui/dist/` |
| `make build-all` | Build frontend + Go binary with embedded UI |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run all tests (verbose, no caching) |
| `make lint` | Run `go vet` |
| `make fmt` | Format code with `gofmt` |

## License

See [LICENSE](LICENSE) for details.
