# Test: TechPreview Network Policies Correlation

## Metadata
- Author: memodi
- Priority: High
- CaseID: 53
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
NetworkPolicy feature enabled.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Patch the FlowCollector to enable `NetworkPolicy` and set
   `networkPolicy.enable=true`.
2. Create a NetworkPolicy in the test-server namespace that allows
   ingress from the test-client namespace using template
   `templates/netobserv/networking/networkPolicy.yaml`.
3. Wait for eBPF agents to restart and collect policy-correlated flows.
4. Query Loki for flows with `NetworkEvents` field populated.
5. Verify that flow records contain NetworkEvent entries with fields:
   Action (Allow/Deny), Type (NetworkPolicy/AdminNetworkPolicy),
   Name, Namespace, Direction.

## Verify
- Flow records contain `NetworkEvents` array with policy correlation data.
- NetworkEvent.Action correctly reflects Allow for permitted traffic.
- NetworkEvent.Name matches the applied NetworkPolicy name.
- NetworkEvent.Direction is set correctly (Ingress/Egress).

## Cleanup
Delete the NetworkPolicy.
Remove the NetworkPolicy feature from FlowCollector.
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
