# SDT Quickstart Guide

SDT (Spec-Driven Testing) is a product-agnostic, AI-powered test framework.
Tests are written as Markdown specs in natural language. An LLM agent reads
each spec, plans the execution steps, and runs them autonomously against any
target system via MCP tools.

## Prerequisites

- Go 1.21+
- An Anthropic API key **or** Google Vertex AI credentials for Claude
- Docker (optional, for Kiwi TCMS)

## Install

```bash
git clone https://github.com/sdt-project/sdt.git
cd sdt
make build        # builds binary to bin/sdt
make install      # or install to $GOPATH/bin
```

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | Yes* | Anthropic API key |
| `ANTHROPIC_VERTEX_PROJECT_ID` | Yes* | Vertex AI project (alternative to API key) |
| `CLOUD_ML_REGION` | Yes* | Vertex AI region (with Vertex) |
| `SDT_PROVIDER` | No | LLM provider: `claude` (default) or `gemini` |
| `SDT_MODEL` | No | Model override (e.g. `claude-sonnet-4-20250514`) |
| `SDT_LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` (default: `info`) |

\* Either `ANTHROPIC_API_KEY` or the Vertex pair is required.

## Set Up SDT in Your Project

```bash
cd my-project
sdt setup myapp
```

This creates:

```
my-project/
  .sdt.yaml              # Configuration
  sdt/
    specs/                # Test specifications
      _suite.md           # Suite-level hooks
      smoke-test.md       # Sample test
    fixtures/             # Fixture definitions
      sample.yaml
    tools/                # Custom tool definitions (YAML)
    mcp/                  # Third-party MCP server guide
      README.md
```

## Add Tools

SDT ships with core tools (shell, file I/O, Python, Node, Go). Add project-specific tools via the default MCP server or third-party MCP servers.

### Default MCP server (YAML tool definitions)

```bash
# Create a draft tool
sdt tools add check_health \
  --description "Check application health" \
  --command "curl -sf {{.endpoint}}/health"

# Edit the YAML to refine parameters
vi sdt/tools/check_health.yaml

# Test it
sdt tools test check_health --input '{"endpoint": "http://localhost:8080"}'

# Approve for use in test runs
sdt tools approve check_health
```

The generated YAML:

```yaml
name: check_health
description: Check application health
category: myapp
status: approved
input:
  endpoint:
    type: string
    description: Base URL of the API
    required: true
command: curl -sf {{.endpoint}}/health
```

### Third-party MCP servers

For complex tools, write an MCP server in any language and configure it in `.sdt.yaml`:

```yaml
mcpServers:
  myapp:
    command: python
    args: [sdt/mcp/myapp_server.py]
    env:
      API_URL: http://localhost:8080
```

### List all tools

```bash
sdt tools list
```

```
NAME            SOURCE          STATUS    DESCRIPTION
shell           core            -         Execute a local shell command
read_file       core            -         Read the contents of a local file
check_health    default         approved  Check application health
myapp_deploy    mcp:myapp       -         Deploy application version
```

## Writing a Test Spec

A spec is a Markdown file with a fixed structure:

```markdown
# Test: Verify API Health

## Metadata
- Author: yourname
- Priority: High
- Labels: [Smoke]
- Timeout: 5m

## Setup
Verify the application server is running.

## Steps
1. Call the health endpoint and verify it returns HTTP 200.
2. Verify the response contains "status": "healthy".
3. Check that response time is under 500ms.

## Verify
- Health endpoint returned 200.
- Response body indicates healthy status.
- Response time is acceptable.

## Cleanup
No cleanup required.
```

### Sections

| Section | Purpose |
|---|---|
| `# Test:` | Test name (required) |
| `## Metadata` | Author, priority, labels, timeout, group, fixtures |
| `## Setup` | Pre-test resource creation |
| `## Steps` | Actions the agent will execute |
| `## Verify` | Assertions the agent checks after steps |
| `## Cleanup` | Resource teardown |

### Metadata Fields

| Field | Type | Description |
|---|---|---|
| `Author` | string | Spec author |
| `Priority` | string | `Critical`, `High`, `Medium`, `Low` |
| `CaseID` | string | Kiwi TCMS test case ID |
| `Labels` | list | Tags like `[Serial]`, `[Disruptive]`, `[Slow]` |
| `Timeout` | duration | Go duration, e.g. `15m`, `1h30m` |
| `Group` | string | Group name (matches `_group_<name>.md`) |
| `Fixtures` | list | Fixture names from fixtures directory |

## Suite Structure

Organize specs in a directory with optional suite/group files:

```
sdt/specs/
  _suite.md              # Suite-level hooks (pre-suite, post-suite, pre-test, post-test)
  _group_database.md     # Group hooks for tests with Group: database
  _group_api.md          # Group hooks for tests with Group: api
  login-test.md          # Test spec
  user-crud.md           # Test spec (Group: database)
  search-api.md          # Test spec (Group: api)
```

Execution order for each test:

```
Suite Pre-Suite (once)
  → Pre-Suite Validation
    Suite Pre-Test
      → Pre-Test Validation
        Group Pre-Test
          → Group Pre-Test Validation
            Setup → Steps → Verify → Cleanup
          Group Post-Test
        Suite Post-Test
Suite Post-Suite (once)
```

Validation sections define conditions that must be true after hooks run.
Hook errors are tolerated, but validation failures abort execution.

## Running Tests

```bash
# Run all specs in a directory
sdt run sdt/specs/

# Run a single spec
sdt run sdt/specs/smoke-test.md

# Dry run (plan only, no execution)
sdt run --dry-run sdt/specs/smoke-test.md

# Skip cached plans
sdt run --no-cache sdt/specs/

# Custom timeout
sdt run --timeout 1h sdt/specs/

# JUnit XML output for CI
sdt run --junit-dir results/ sdt/specs/

# Extra context for the LLM
sdt run --context "API is running on port 3000" sdt/specs/

# Debug logging
SDT_LOG_LEVEL=debug sdt run sdt/specs/smoke-test.md
```

## Other Commands

```bash
# List all specs
sdt list sdt/specs/

# Validate spec structure (no LLM needed)
sdt validate sdt/specs/

# Review spec quality with AI
sdt review sdt/specs/smoke-test.md

# Cache management
sdt cache status sdt/specs/
sdt cache clear

# List registered projects
sdt projects
```

## Fixtures

Fixtures are parameterized resource definitions with lifecycle instructions
that the LLM agent follows at runtime:

```yaml
name: test-database
description: PostgreSQL test database
parameters:
  name: testdb
  port: "5432"
lifecycle:
  create: "Create a PostgreSQL database named '${name}' on port ${port}"
  ready: "Verify the database '${name}' accepts connections"
  cleanup: "Drop the database '${name}' and remove all data"
```

Reference a fixture in a spec's metadata:

```markdown
## Metadata
- Fixtures: [test-database]

## Setup
Deploy the test database using fixture `test-database`.
```

## Configuration Reference

### .sdt.yaml

```yaml
project: myapp
specsDir: sdt/specs
fixturesDir: sdt/fixtures
toolsDir: sdt/tools

mcpServers:
  myapp:
    command: python
    args: [-m, myapp_mcp_server]
    env:
      API_URL: http://localhost:8080
```

### CLI Flags for `sdt run`

| Flag | Default | Description |
|---|---|---|
| `--timeout` | `30m` | Default timeout per spec |
| `--junit-dir` | | JUnit XML output directory |
| `--no-cache` | `false` | Force re-planning |
| `--dry-run` | `false` | Plan only, no execution |
| `--fixtures-dir` | `fixtures` | Override fixtures directory |
| `--context` | | Extra LLM context |
| `--skip-cleanup` | `false` | Skip cleanup phases |
| `--skip-phases` | | Phases to skip |
| `--only-phases` | | Run only these phases |
| `--project` | | Explicit project selection |
| `--tcms` | `false` | Report to Kiwi TCMS |
| `--tcms-product` | | TCMS product name |
| `--tcms-build` | `unspecified` | TCMS build name |
| `--tcms-run-id` | | Existing TCMS run ID |
| `--tcms-plan-id` | | TCMS plan ID (filters specs) |
