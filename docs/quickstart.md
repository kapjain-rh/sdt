# SDT Quickstart Guide

SDT (Spec-Driven Testing) is an AI-powered test framework for OpenShift.
Tests are written as Markdown specs in natural language. An LLM agent reads
each spec, plans the execution steps, and runs them autonomously against a
live OpenShift cluster.

## Prerequisites

- Go 1.21+
- `oc` CLI logged into an OpenShift cluster (`oc whoami` should succeed)
- An Anthropic API key **or** Google Vertex AI credentials for Claude
- Docker (optional, for Kiwi TCMS)

## Install

```bash
git clone https://github.com/openshift/sdt.git
cd sdt
go build -o bin/sdt ./cmd/sdt/
```

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | Yes* | Anthropic API key |
| `ANTHROPIC_VERTEX_PROJECT_ID` | Yes* | Vertex AI project (alternative to API key) |
| `CLOUD_ML_REGION` | Yes* | Vertex AI region (with Vertex) |
| `SDT_MODEL` | No | Model override (e.g. `claude-sonnet-4-20250514`) |
| `SDT_LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` (default: `info`) |
| `KUBECONFIG` | No | Path to kubeconfig (default: `~/.kube/config`) |

\* Either `ANTHROPIC_API_KEY` or the Vertex pair is required.

## Writing a Test Spec

A spec is a Markdown file with a fixed structure:

```markdown
# Test: Verify My Feature

## Metadata
- Author: yourname
- Priority: High
- CaseID: 12345
- Labels: [Serial]
- Timeout: 15m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Verify all pods in `openshift-netobserv` namespace are Running.
2. Check that the console plugin deployment is available.

## Verify
- All pods are Running with all containers ready.
- FlowCollector CR status is Ready.

## Cleanup
Delete the FlowCollector CR and wait for all pods to terminate.
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
| `Fixtures` | list | Fixture names from `fixtures/` directory |

## Suite Structure

Organize specs in a directory with optional suite/group files:

```
specs/myproduct/
  _suite.md              # Suite-level hooks (pre-suite, post-suite, pre-test, post-test)
  _group_with_loki.md    # Group hooks for tests with Group: with-loki
  _group_with_kafka.md   # Group hooks for tests with Group: with-kafka
  sanity.md              # Test spec
  alerts.md              # Test spec
  upgrade.md             # Test spec
```

- `_suite.md` defines `Pre-Suite`, `Pre-Test`, `Post-Test`, `Post-Suite` hooks
  that run around every test.
- `_group_<name>.md` defines `Pre-Test` / `Post-Test` hooks for tests in that group.

Execution order for each test:

```
Suite Pre-Suite  (once)
  Suite Pre-Test
    Group Pre-Test
      Setup → Steps → Verify → Cleanup
    Group Post-Test
  Suite Post-Test
Suite Post-Suite (once)
```

## Running Tests

```bash
# Run all specs in a directory
sdt run specs/myproduct/

# Run a single spec
sdt run specs/myproduct/sanity.md

# Dry run (plan only, no execution)
sdt run --dry-run specs/myproduct/sanity.md

# Skip cached plans
sdt run --no-cache specs/myproduct/sanity.md

# Custom timeout
sdt run --timeout 1h specs/myproduct/

# JUnit XML output
sdt run --junit-dir results/ specs/myproduct/

# Debug logging
SDT_LOG_LEVEL=debug sdt run specs/myproduct/sanity.md
```

## Other Commands

```bash
# List all specs
sdt list specs/myproduct/
sdt list --format json specs/myproduct/

# Validate spec structure (no cluster or LLM needed)
sdt validate specs/myproduct/

# Review spec quality with AI
sdt review specs/myproduct/sanity.md

# Cache management
sdt cache status specs/myproduct/
sdt cache clear
sdt cache clear specs/myproduct/sanity.md
```

## Fixtures

Fixtures are parameterized resource definitions in `fixtures/`. Each fixture
has a template path, parameters, and lifecycle instructions that the LLM agent
follows.

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

Reference a fixture in a spec's metadata:

```markdown
## Metadata
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
```

## Kiwi TCMS Integration

SDT integrates with [Kiwi TCMS](https://kiwitcms.org) for test case management
and result reporting.

### Start a Local Kiwi TCMS Instance

```bash
docker compose -f docker-compose.kiwi.yml up -d
```

This starts Kiwi TCMS at `https://localhost:8443` with a MariaDB backend.

First-time setup:

```bash
# Run database migrations
docker exec sdt-kiwi-tcms /Kiwi/manage.py migrate --run-syncdb

# Create admin user
docker exec sdt-kiwi-tcms /Kiwi/manage.py createsuperuser \
  --username admin --email admin@example.com --noinput

# Set password
docker exec sdt-kiwi-tcms /Kiwi/manage.py shell -c \
  "from django.contrib.auth.models import User; u = User.objects.get(username='admin'); u.set_password('admin'); u.save()"
```

Then create a Product, Version, and Category via the web UI at
`https://localhost:8443`, or via JSON-RPC:

```bash
curl -sk https://localhost:8443/json-rpc/ \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Classification.create","params":[{"name":"OpenShift"}],"id":1}'

curl -sk https://localhost:8443/json-rpc/ \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Product.create","params":[{"name":"NetObserv","classification":1}],"id":2}'

curl -sk https://localhost:8443/json-rpc/ \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Version.create","params":[{"value":"4.18","product":1}],"id":3}'

curl -sk https://localhost:8443/json-rpc/ \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Category.create","params":[{"name":"Functional","product":1}],"id":4}'
```

### Set TCMS Environment Variables

```bash
export KIWI_TCMS_URL=https://localhost:8443
export KIWI_TCMS_USERNAME=admin
export KIWI_TCMS_PASSWORD=admin
```

### Sync Specs to TCMS

Upload all specs as test cases and create a test plan:

```bash
sdt tcms sync --product NetObserv specs/netobserv/
```

This will:
- Create (or update) a test case in Kiwi TCMS for each spec
- Create a test plan named "SDT - <suite name>"
- Add all cases to the plan
- Update each spec file's `CaseID` field with the Kiwi TCMS case ID

### Check Linkage

Verify that spec CaseIDs map to existing Kiwi TCMS test cases:

```bash
sdt tcms status specs/netobserv/
```

Output:

```
[+] Sanity Test NetObserv (CaseID: 58) — linked
[~] My New Test (CaseID: 99999) — Case 99999 not found in Kiwi TCMS
[?] Untracked Test (CaseID: ) — No CaseID in spec metadata
```

### Run Tests with TCMS Reporting

Run tests and report results to Kiwi TCMS in real-time:

```bash
# Create a new test run automatically
sdt run --tcms --tcms-product NetObserv specs/netobserv/

# Report to an existing test run
sdt run --tcms --tcms-run-id 42 specs/netobserv/

# With a custom build name
sdt run --tcms --tcms-product NetObserv --tcms-build "4.18-nightly" specs/netobserv/
```

Results appear in the Kiwi TCMS web UI as PASSED, FAILED, or WAIVED per test
execution within the test run.

### Import Test Cases from TCMS

Generate spec stubs from an existing Kiwi TCMS test plan:

```bash
sdt tcms import --plan-id 1 --output specs/imported/
```

### Stop Kiwi TCMS

```bash
docker compose -f docker-compose.kiwi.yml down      # stop containers
docker compose -f docker-compose.kiwi.yml down -v    # stop + delete data
```

## How It Works

When you run `sdt run`, the following pipeline executes for each spec:

1. **Memory Agent** checks the plan cache for a previously generated plan.
2. **Planner Agent** reads the spec and creates a step-by-step execution plan
   (or uses the cached plan).
3. **Executor Agent** runs each plan step in auto-pilot mode using tools:
   `oc_run`, `oc_apply`, `oc_get`, `oc_delete`, `process_template`,
   `wait_for_condition`, `wait_for_pods_ready`, `shell`, etc.
4. Each step is validated after execution.
5. Results are reported to the console, JUnit XML, and/or Kiwi TCMS.

The agent loop sends messages to Claude, receives tool call requests, executes
them against the cluster, and returns results — repeating until the agent
signals completion.
