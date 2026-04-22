# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is SDT?

SDT (Spec-Driven Testing) is a product-agnostic, AI-powered test framework. Tests are written as Markdown specs in natural language. An LLM agent reads each spec, plans execution steps, and runs them autonomously against a target system via MCP tools. The framework is extensible — project-specific tools, constraints, and prompts are registered by project packages (e.g., `projects/openshift/` for OpenShift testing).

## Build and Development Commands

```bash
make build          # Build binary to bin/sdt
make install        # Install to $GOPATH/bin
make test           # Run all tests (verbose, no caching)
make test-short     # Run tests in short mode
make lint           # Run go vet
make fmt            # Format code with gofmt
make fmt-check      # Check formatting (CI-friendly)
```

Run a single test:
```bash
go test ./pkg/cache/... -v -count=1 -run TestStorePlanCaching
```

The binary version is injected via `-ldflags "-X main.version=..."` from `git describe`.

## Environment Variables

- `SDT_PROVIDER` — LLM provider: `claude` (default) or `gemini`
- `ANTHROPIC_API_KEY` or (`ANTHROPIC_VERTEX_PROJECT_ID` + `CLOUD_ML_REGION`) — Claude credentials
- `GOOGLE_CLOUD_PROJECT` + `GOOGLE_CLOUD_REGION` — Gemini credentials (also accepts `ANTHROPIC_VERTEX_PROJECT_ID` + `CLOUD_ML_REGION`)
- `SDT_MODEL` — override model name (default: `claude-opus-4-6` for Claude, `gemini-2.5-pro` for Gemini)
- `SDT_LOG_LEVEL` — `debug`, `info`, `warn`, `error`
- `KUBECONFIG` — path to kubeconfig for `oc` commands
- `KIWI_TCMS_URL`, `KIWI_TCMS_USERNAME`, `KIWI_TCMS_PASSWORD` — Kiwi TCMS integration

## Architecture

### Agent Pipeline

`sdt run` executes a multi-agent pipeline for each spec:

1. **Memory Agent** (`pkg/agent/memory.go`) — checks plan cache for a previously generated plan
2. **Planner Agent** (`pkg/agent/planner.go`) — sends spec to Claude, receives a JSON `ExecutionPlan` with phases/steps/tool mappings
3. **Executor Agent** (`pkg/agent/executor.go`) — runs the plan via Claude's agentic tool-use loop (`RunAgentLoop`), calling MCP tools autonomously
4. **Reviewer Agent** (`pkg/agent/reviewer.go`) — reviews spec quality or analyzes execution failures
5. **Coding Agent** (`pkg/agent/coding.go`) — generates YAML templates from natural language

System prompts for all agents live in `pkg/agent/prompts.go`. Dynamic prompt generation combines base prompts with registered tools, constraints, and project context via `pkg/agent/prompt_builder.go` (`BuildSystemPrompt`).

### LLM Client (`pkg/llm/`)

Uses a `Provider` interface to support multiple LLM backends. `Client` wraps the active provider with shared logic (`Chat`, `RunAgentLoop`). Providers:
- **Claude** (`claude.go`) — Anthropic Messages API (direct or Vertex AI)
- **Gemini** (`gemini.go`) — Google Gemini via Vertex AI `generateContent` endpoint

The key method is `RunAgentLoop` — a multi-turn loop that sends messages, executes tool calls via a handler function, sends results back, and repeats until `end_turn`. Provider selection is via `SDT_PROVIDER` env var.

### Tool Registry (`pkg/tools/`)

Framework-level tool infrastructure. Each tool has a `ToolHandler` function, a JSON schema for input, and returns `*ToolResult`. The `Tool` struct includes `PromptHint` and `Category` fields for prompt generation.

**Core tools** (registered via `RegisterCoreTools()`):
- **Shell tools** (`shell.go`): `shell`, `read_file`, `write_file`
- **Runtime tools** (`runtime.go`): `python`, `node`, `npm`, `npx`, `go_run`, `cypress`, `check_runtimes`

**Constraints** (`constraints.go`): Centralized `ToolConstraints` system with `Check()`, `CheckShell()`, `BlockShellCommand()`, `RedirectTool()` helpers. Framework ships generic constraints (block filesystem root search, block sleep loops). Projects add their own via `AddConstraint()`.

### Project System (`pkg/project/`, `pkg/config/`)

Projects are pluggable integrations that register tools, constraints, and prompts for a specific target system. The `Project` interface (`pkg/project/project.go`) defines: `Name()`, `Description()`, `Setup(registry, constraints)`, `PromptFragment()`, `SystemDescription()`, `Detect()`.

Projects self-register via `init()` using `project.Register()` (same pattern as `database/sql` drivers). The CLI selects the active project via:
1. `--project` flag (highest priority)
2. `.sdt.yaml` config file (`project:` field)
3. Auto-detection (checks which project's prerequisites are available)

Run `sdt projects` to list available projects and their detection status.

**Config file** (`.sdt.yaml` in working directory, optional):
```yaml
project: openshift
specsDir: specs/
fixturesDir: fixtures/
```

### Project Tools (`projects/`)

Project-specific tools live in separate packages under `projects/`. Each project implements `project.Project` and registers via `init()`.

**OpenShift** (`projects/openshift/`):
- `project.go`: `OpenShiftProject` implementing `project.Project`, auto-registers via `init()`
- `tools.go`: `OCClient`, `oc_run`, `oc_apply`, `oc_delete`, `oc_get`, `oc_patch`, `oc_exec`, `oc_logs`, `create_namespace`, `delete_namespace`, `wait_for_condition`, `wait_for_pods_ready`, `deploy_operator`, `process_template`, `query_metric`
- `constraints.go`: `AddOpenShiftConstraints()` — blocks shell usage of oc subcommands, redirects oc_run wait to wait_for_condition
- `prompts.go`: `PromptFragment` — OpenShift-specific LLM guidance

**Adding a new project** — two paths:

1. **External (MCP-based):** Run `sdt setup <name>` from any project directory. Define tools as MCP servers (any language), configure in `.sdt.yaml` under `mcpServers`. No Go code or SDT recompilation needed.

2. **Compiled-in (Go-based):** Run `sdt init <name>` from the SDT source tree to scaffold Go code under `projects/<name>/`, then add a blank import in `cmd/sdt/run.go`. Use this when you need complex tool logic tightly integrated with SDT.

### MCP Client (`pkg/mcp/client.go`, `pkg/mcp/bridge.go`)

SDT acts as an MCP client, connecting to external MCP servers defined in `.sdt.yaml`. On startup, it spawns each server subprocess, performs the JSON-RPC 2.0 handshake (`initialize`), discovers tools via `tools/list`, and registers them into the tool registry. Tool calls are proxied to the server via `tools/call`.

MCP servers can be written in any language (Go, Python, TypeScript, bash) using the standard MCP protocol over stdio. This is the primary mechanism for adding project-specific tools without modifying SDT source.

Config example:
```yaml
mcpServers:
  myapp:
    command: python
    args: [-m, myapp_mcp_server]
    env:
      API_URL: http://localhost:8080
```

### Spec System (`pkg/spec/`)

Markdown test specs are parsed into `TestSpec` structs with sections: Metadata, Setup, Steps, Verify, Cleanup. Directory structure:

- `_suite.md` — suite-level hooks (pre-suite, pre-test, post-test, post-suite) and optional validation sections
- `_group_<name>.md` — group-level hooks for tests with matching `Group:` metadata
- `*.md` (no underscore prefix) — individual test specs

Validation sections (`## Pre-Suite Validation`, `## Pre-Test Validation`) define conditions that must be true after hooks run. Hook step errors are tolerated (stopOnError=false), but validation failures abort the suite/test. This separates "best-effort setup" from "required preconditions".

Execution order: Suite Pre-Suite → Pre-Suite Validation → (Suite Pre-Test → Suite Pre-Test Validation → Group Pre-Test → Group Pre-Test Validation → Setup → Steps → Verify → Cleanup → Group Post-Test → Suite Post-Test) → Suite Post-Suite.

### Runner (`pkg/runner/runner.go`)

Orchestrates the full lifecycle. `RunSuite` iterates test specs; `RunSpec` wires up agents, resolves fixtures, runs planning/execution, and collects results. Hook phases are also executed through the LLM agent loop. Configurable via `Config` struct: `SystemDescription` (e.g., "OpenShift cluster"), `ProjectContext` (prompt fragment), `Constraints`, `SkipPhases`/`OnlyPhases` for phase filtering.

### Fixtures (`pkg/fixture/`, `fixtures/`)

YAML definitions with `name`, `template`, `parameters`, and `lifecycle` (create/ready/cleanup as natural language). The LLM interprets lifecycle instructions to generate tool calls. Specs reference fixtures by name in metadata.

### Templates (`templates/`)

Parameterized Kubernetes/OpenShift YAML templates processed via `oc process` or Go `text/template`. Organized by component (netobserv, kafka, loki, operators, etc.).

### Cache (`pkg/cache/`, `.sdt/cache/`)

Plans are cached by spec content hash. Results are stored per-spec with timestamps. Cache lives in `.sdt/cache/` (gitignored).

### Reporters (`pkg/reporter/`)

`ConsoleReporter` and `JUnitReporter` implement the `Reporter` interface. `MultiReporter` fans out to multiple reporters. TCMS reporter (`pkg/tcms/reporter.go`) reports to Kiwi TCMS.

### TCMS Integration (`pkg/tcms/`)

JSON-RPC client for Kiwi TCMS. Syncs specs as test cases, creates test plans/runs, and reports execution results. Accessed via `sdt tcms sync|status|import` commands.
