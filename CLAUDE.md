# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is SDT?

SDT (Spec-Driven Testing) is an AI-powered test framework for OpenShift. Tests are written as Markdown specs in natural language. An LLM agent (Claude) reads each spec, plans execution steps, and runs them autonomously against a live OpenShift cluster via MCP tools.

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

System prompts for all agents live in `pkg/agent/prompts.go`.

### LLM Client (`pkg/llm/`)

Uses a `Provider` interface to support multiple LLM backends. `Client` wraps the active provider with shared logic (`Chat`, `RunAgentLoop`). Providers:
- **Claude** (`claude.go`) — Anthropic Messages API (direct or Vertex AI)
- **Gemini** (`gemini.go`) — Google Gemini via Vertex AI `generateContent` endpoint

The key method is `RunAgentLoop` — a multi-turn loop that sends messages, executes tool calls via a handler function, sends results back, and repeats until `end_turn`. Provider selection is via `SDT_PROVIDER` env var.

### Tool Registry (`pkg/tools/`)

MCP tools that the LLM agent can call during execution. All tools are registered via `RegisterAllTools()`:

- **OC tools** (`oc.go`): `oc_run`, `oc_apply`, `oc_delete`, `oc_get`, `oc_patch`, `oc_exec`, `oc_logs`
- **Resource tools** (`resource.go`): `create_namespace`, `delete_namespace`, `wait_for_condition`, `wait_for_pods_ready`
- **Operator tools** (`operator.go`): `deploy_operator`, `process_template`
- **Metrics tools** (`metrics.go`): `query_metric` (PromQL via thanos-querier)
- **Shell tools** (`shell.go`): `shell`, `read_file`, `write_file`

Each tool has a `ToolHandler` function, a JSON schema for input, and returns `*ToolResult`.

### Spec System (`pkg/spec/`)

Markdown test specs are parsed into `TestSpec` structs with sections: Metadata, Setup, Steps, Verify, Cleanup. Directory structure:

- `_suite.md` — suite-level hooks (pre-suite, pre-test, post-test, post-suite) and optional validation sections
- `_group_<name>.md` — group-level hooks for tests with matching `Group:` metadata
- `*.md` (no underscore prefix) — individual test specs

Validation sections (`## Pre-Suite Validation`, `## Pre-Test Validation`) define conditions that must be true after hooks run. Hook step errors are tolerated (stopOnError=false), but validation failures abort the suite/test. This separates "best-effort setup" from "required preconditions".

Execution order: Suite Pre-Suite → Pre-Suite Validation → (Suite Pre-Test → Suite Pre-Test Validation → Group Pre-Test → Group Pre-Test Validation → Setup → Steps → Verify → Cleanup → Group Post-Test → Suite Post-Test) → Suite Post-Suite.

### Runner (`pkg/runner/runner.go`)

Orchestrates the full lifecycle. `RunSuite` iterates test specs; `RunSpec` wires up agents, resolves fixtures, runs planning/execution, and collects results. Hook phases are also executed through the LLM agent loop.

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
