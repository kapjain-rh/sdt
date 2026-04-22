# Test: NetObserv with eBPF Manager

## Metadata
- Author: memodi
- Priority: High
- CaseID: 40
- Labels: [Serial, Slow]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Verify the eBPF manager integration is functional by checking the
   eBPF agent pods are using the manager-based loading mechanism.
2. Wait for flow data to be collected in Loki.
3. Query Loki for flows between test-client and test-server.
4. Verify that flows are captured correctly with eBPF manager.

## Verify
- eBPF agent pods are running with the manager integration.
- Flows are captured correctly and stored in Loki.
- No errors in eBPF agent logs related to the eBPF manager.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
