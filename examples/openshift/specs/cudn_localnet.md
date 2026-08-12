# Test: Validate CUDN with Localnet

## Metadata
- Author: memodi
- Priority: High
- CaseID: 37
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
SecondaryNetworks support enabled.

## Steps
1. Create a ClusterUserDefinedNetwork (CUDN) with Localnet topology.
2. Create namespaces and apply the CUDN via namespace label selectors.
3. Deploy pods attached to the CUDN Localnet network.
4. Generate traffic between CUDN pods.
5. Query Loki for flows from the CUDN-attached pods.
6. Verify that flow records correctly reference the CUDN network.

## Verify
- Flow records capture traffic on the CUDN Localnet network.
- Network name fields correctly identify the CUDN.

## Cleanup
Delete the CUDN pods and namespaces.
Delete the ClusterUserDefinedNetwork.
Delete the FlowCollector CR.
