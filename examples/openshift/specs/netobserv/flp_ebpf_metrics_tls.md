# Test: Verify FLP, eBPF and Console Metrics with TLS Auto

## Metadata
- Author: memodi
- Priority: High
- CaseID: 43
- Labels: [Serial]
- Timeout: 20m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy a FlowCollector with processor.metrics.server.tls.type=Auto and
agent.ebpf.metrics.server.tls.type=Auto.

## Steps
1. Verify the FLP servicemonitor scheme is `https` and the serverName
   matches the expected service.
2. Verify the eBPF agent servicemonitor scheme is `https` and the serverName
   matches the expected service.
3. Query Prometheus for `sum(netobserv_ingest_flows_processed)` and verify
   it is greater than 0.
4. Query Prometheus for `sum(netobserv_agent_exported_batch_total)` and
   verify it is greater than 0.
5. Verify the console plugin metrics are available by querying for the
   console plugin service metrics endpoint.

## Verify
- FLP servicemonitor uses `https` scheme with correct serverName for TLS Auto.
- eBPF servicemonitor uses `https` scheme with correct serverName for TLS Auto.
- FLP and eBPF Prometheus metrics are collected successfully over TLS.
- Console plugin metrics endpoint is accessible.

## Cleanup
Delete the FlowCollector CR.
