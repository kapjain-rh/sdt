# Test: Verify PacketTranslation Feature

## Metadata
- Author: memodi
- Priority: High
- CaseID: 55
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
PacketTranslation eBPF feature enabled.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Patch the FlowCollector to enable the `PacketTranslation` eBPF feature:
   set `agent.ebpf.features` to include `PacketTranslation`.
2. Wait for the eBPF agent pods to restart with the new feature.
3. Generate traffic through services to trigger NAT translation.
4. Query Loki for flows that have NAT/translation applied.
5. Verify that `XlatSrcAddr`, `XlatDstAddr`, `XlatSrcK8S_Name`,
   `XlatDstK8S_Name` fields are populated in the flow records.

## Verify
- Flow records contain populated `Xlat*` translation fields.
- Translated addresses map correctly to Kubernetes resources.

## Cleanup
Remove the PacketTranslation feature from FlowCollector.
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
