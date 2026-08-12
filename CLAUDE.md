# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is SDT?

SDT (Spec-Driven Testing) is a product-agnostic, AI-powered test framework. Tests are written as Markdown specs in natural language. An LLM agent reads each spec, plans execution steps, and runs them autonomously against a target system via MCP tools. Project-specific tools are provided externally via MCP servers or YAML tool definitions — no compiled-in projects.

## Build and Development Commands

```bash
make build          # Build binary to bin/sdt
make build-ui       # Build frontend static export into pkg/api/ui/dist/ (requires Node.js 18+)
make build-all      # Build frontend + Go binary with embedded UI
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

### UI Development

The web UI is a Next.js app in `ui/`. For frontend development, run the API and frontend dev server separately:

```bash
sdt serve --project-dir /path/to/project --port 8090   # API server
cd ui && npm install && npm run dev                     # Frontend on :3000, proxies API to :8090
```

Set `SDT_API_PORT` to use a different backend port. The AGENTS.md in `ui/` warns that this Next.js version has breaking API changes — always read `node_modules/next/dist/docs/` before writing frontend code.

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

The key method is `RunAgentLoop` — a multi-turn loop that sends messages, executes tool calls via a handler function, sends results back, and repeats until `end_turn` (max 50 iterations, 5 consecutive error limit). Provider selection is via `SDT_PROVIDER` env var; `NewClient()` auto-detects from env vars unless a provider is injected via `WithProvider()`.

### Tool Registry (`pkg/tools/`)

Framework-level tool infrastructure. Each tool has a `ToolHandler` function, a JSON schema for input, and returns `*ToolResult`. The `Tool` struct includes `PromptHint` and `Category` fields for prompt generation.

**Core tools** (registered via `RegisterCoreTools()` in `toolset.go`):
- **Shell tools** (`shell.go`): `shell`, `read_file`, `write_file`
- **Runtime tools** (`runtime.go`): `python`, `node`, `npm`, `npx`, `go_run`, `cypress`, `check_runtimes`

**Custom YAML tools** (`custom.go`): `LoadCustomTools()` reads YAML tool definitions from a directory (e.g., `sdt/tools/*.yaml`). Each `CustomToolDef` has a Go template `Command` field, a `Status` (draft/approved), and typed `Input` parameters. Only approved tools are loaded during `sdt run`.

**Constraints** (`constraints.go`): Centralized `ToolConstraints` system with `Check()`, `CheckShell()`, `BlockShellCommand()`, `RedirectTool()` helpers. Framework ships generic constraints (block filesystem root search, block sleep loops). Projects add their own via `AddConstraint()`.

### Configuration (`pkg/config/`)

**Config file** (`.sdt.yaml` in working directory, optional):
```yaml
project: myapp
description: "My application"
context: |
  Use dedicated tools instead of shell commands where available.
specsDir: sdt/specs
fixturesDir: sdt/fixtures
toolsDir: sdt/tools
mcpServers:
  openshift:
    command: ./openshift-mcp-server
    env:
      KUBECONFIG: /path/to/kubeconfig
```

`description` and `context` are passed to the LLM as `SystemDescription` and `ProjectContext` respectively. Tools come from two external sources: MCP servers (configured in `mcpServers`) and YAML tool definitions (in `toolsDir`).

### MCP Client (`pkg/mcp/client.go`, `pkg/mcp/bridge.go`)

SDT acts as an MCP client, connecting to external MCP servers defined in `.sdt.yaml`. On startup, it spawns each server subprocess, performs the JSON-RPC 2.0 handshake (`initialize`), discovers tools via `tools/list`, and registers them into the tool registry via `RegisterMCPTools()` in `bridge.go`. Tool calls are proxied to the server via `tools/call`.

MCP servers can be written in any language (Go, Python, TypeScript, bash) using the standard MCP protocol over stdio. This is the primary mechanism for adding project-specific tools without modifying SDT source. See `examples/openshift-mcp-server/` for a complete Go-based MCP server.

### Spec System (`pkg/spec/`)

Markdown test specs are parsed into `TestSpec` structs with sections: Metadata, Setup, Steps, Verify, Cleanup. Directory structure:

- `_suite.md` — suite-level hooks (pre-suite, pre-test, post-test, post-suite) and optional validation sections
- `_group_<name>.md` — group-level hooks for tests with matching `Group:` metadata
- `*.md` (no underscore prefix) — individual test specs

Validation sections (`## Pre-Suite Validation`, `## Pre-Test Validation`) define conditions that must be true after hooks run. Hook step errors are tolerated (stopOnError=false), but validation failures abort the suite/test. This separates "best-effort setup" from "required preconditions".

Execution order: Suite Pre-Suite → Pre-Suite Validation → (Suite Pre-Test → Suite Pre-Test Validation → Group Pre-Test → Group Pre-Test Validation → Setup → Steps → Verify → Cleanup → Group Post-Test → Suite Post-Test) → Suite Post-Suite.

### Runner (`pkg/runner/runner.go`)

Orchestrates the full lifecycle. `RunSuite` iterates test specs (supports parallel execution); `RunSpec` wires up agents, resolves fixtures, runs planning/execution, and collects results. Hook phases are also executed through the LLM agent loop. Configurable via `Config` struct: `SystemDescription` (e.g., "OpenShift cluster"), `ProjectContext` (prompt fragment), `Constraints`, `SkipPhases`/`OnlyPhases` for phase filtering, `Parallel` for concurrent spec execution, `Retries` for automatic retry on failure.

### API Server (`pkg/api/`)

HTTP server (`sdt serve`) providing a REST API and optional embedded web UI. `Server` in `server.go` wires together:
- `ProjectStore` (`store.go`) — reads specs, fixtures, tools, cache from the project directory
- `ToolExecutor` (`tool_executor.go`) — tests tools with sample input
- `SpecExecutor` (`spec_executor.go`) — runs specs via the agent pipeline
- `MCPManager` (`mcpmanager.go`) — manages MCP server connections for the API

The `--ui` flag serves the bundled Next.js static export from `pkg/api/ui/dist/` (embedded via `go:embed` in `ui.go`).

### Fixtures (`pkg/fixture/`)

YAML definitions with `name`, `template`, `parameters`, and `lifecycle` (create/ready/cleanup as natural language). The LLM interprets lifecycle instructions to generate tool calls. Specs reference fixtures by name in metadata.

### Cache (`pkg/cache/`, `.sdt/cache/`)

Plans are cached by spec content hash. Results are stored per-spec with timestamps. Cache lives in `.sdt/cache/` (gitignored).

### Template System (`pkg/template/`)

`TemplateDef` structs for reusable resource templates with typed parameters. Used by the Coding Agent to generate YAML from natural language descriptions. `processor.go` handles template rendering; `registry.go` manages template discovery and lookup.

### Reporters (`pkg/reporter/`)

`ConsoleReporter` and `JUnitReporter` implement the `Reporter` interface. `MultiReporter` fans out to multiple reporters. TCMS reporter (`pkg/tcms/reporter.go`) reports to Kiwi TCMS.

### TCMS Integration (`pkg/tcms/`)

JSON-RPC client for Kiwi TCMS. Syncs specs as test cases, creates test plans/runs, and reports execution results. Accessed via `sdt tcms sync|status|import` commands.

### CLI Structure (`cmd/sdt/`)

Cobra-based CLI. Each subcommand lives in its own file. Notable commands beyond `run`:
- `serve` — HTTP API server with optional embedded UI
- `watch` — polls spec files and re-validates on save (useful during authoring)
- `diff` — shows the cached execution plan for a spec
- `doctor` — pre-flight checks for LLM credentials, MCP servers, tools, and config
- `specs` — lifecycle management (add, run with section control, approve, list)
- `suite` — suite management (add, run, list)
- `fixtures` — fixture lifecycle (add, validate, approve, list)
- `tools` — tool lifecycle (add, test, approve, list)