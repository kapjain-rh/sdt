# Test: Verify FLP and eBPF Metrics with TLS Disabled

## Metadata
- Author: memodi
- Priority: High
- CaseID: 42
- Labels: [Serial]
- Timeout: 20m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy a FlowCollector with processor.metrics.server.tls.type=Disabled and
agent.ebpf.metrics.server.tls.type=Disabled.

## Steps
1. Verify the FLP servicemonitor exists in `openshift-netobserv` and its
   scheme is set to `http` (not https).
2. Verify the eBPF agent servicemonitor exists in
   `openshift-netobserv-privileged` and its scheme is `http`.
3. Query Prometheus for `sum(netobserv_ingest_flows_processed)` and verify
   the value is greater than 0.
4. Query Prometheus for `sum(netobserv_agent_exported_batch_total)` and
   verify the value is greater than 0.
5. Check the FLP health endpoint returns healthy status.
6. Check the eBPF agent health endpoints return healthy status.

## Verify
- FLP servicemonitor scheme is `http` when TLS is disabled.
- eBPF servicemonitor scheme is `http` when TLS is disabled.
- FLP metrics (netobserv_ingest_flows_processed) are scraped by Prometheus.
- eBPF metrics (netobserv_agent_exported_batch_total) are scraped by Prometheus.

## Cleanup
Delete the FlowCollector CR.
