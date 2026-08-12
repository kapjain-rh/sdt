# Test: Verify eBPF Agent Filtering

## Metadata
- Author: memodi
- Priority: High
- CaseID: 39
- Labels: [Serial, Slow]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
EBPFFilterEnable=true.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Patch the FlowCollector to add a filter rule that accepts only traffic
   from the test-client namespace CIDR.
2. Wait for the eBPF agent pods to restart with the new filter config.
3. Query Prometheus for `rate(netobserv_agent_filtered_flows_total{reason="FilterAccept"}[1m])`
   and verify it is greater than 0.
4. Query Prometheus for `rate(netobserv_agent_filtered_flows_total{reason="FilterNoMatch"}[1m])`
   and verify it is greater than 0 (background traffic should be filtered out).
5. Add a reject filter rule for specific traffic and verify
   `FilterReject` metric increases.
6. Query Loki and verify that only flows matching the accept rule are stored.

## Verify
- FilterAccept metric shows accepted flows matching the filter.
- FilterNoMatch metric shows non-matching background traffic is dropped.
- FilterReject metric increases when reject rules are active.
- Loki contains only flows that passed the filter rules.

## Cleanup
Remove the filter rules from the FlowCollector.
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
