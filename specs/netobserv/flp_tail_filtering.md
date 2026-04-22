# Test: Verify FLP Tail-Based Filtering

## Metadata
- Author: memodi
- Priority: High
- CaseID: 44
- Labels: [Serial]
- Timeout: 20m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Patch the FlowCollector to add FLP processor filters that drop
   flows with Bytes < 100.
2. Wait for FLP pods to restart with the new filter configuration.
3. Generate a mix of small and large traffic flows.
4. Query Loki for all flows from the test namespaces.
5. Verify that only flows with Bytes >= 100 are stored in Loki.

## Verify
- Flows with Bytes < 100 are not present in Loki (filtered at FLP level).
- Flows with Bytes >= 100 are present in Loki.
- FLP processor correctly applies tail-based sampling filters.

## Cleanup
Remove the FLP filters from FlowCollector.
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
