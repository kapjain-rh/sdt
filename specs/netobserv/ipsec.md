# Test: Verify IPSec Feature

## Metadata
- Author: memodi
- Priority: High
- CaseID: 46
- Labels: [Disruptive]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.
Ensure the cluster has IPSec enabled on the network configuration.

## Steps
1. Verify that the cluster network configuration has IPSec enabled.
   Skip the test if IPSec is not available.
2. Wait for flow data to be collected in Loki.
3. Query Loki for flows between test-client and test-server.
4. Verify that the `IPSecStatus` field is populated in flow records,
   indicating whether the traffic was encrypted via IPSec.

## Verify
- Flow records contain a non-empty `IPSecStatus` field.
- IPSec status correctly reflects the encryption state of the traffic.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
