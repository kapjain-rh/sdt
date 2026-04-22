# Test: Validate UDN with NetObserv

## Metadata
- Author: memodi
- Priority: High
- CaseID: 64
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
SecondaryNetworks support enabled.

## Steps
1. Patch the FlowCollector to enable secondary networks:
   set `processor.advanced.secondaryNetworks` to include UDN interfaces.
2. Create a UserDefinedNetwork (UDN) in a test namespace.
3. Deploy pods attached to the UDN.
4. Generate traffic between UDN-attached pods.
5. Query Loki for flows from the UDN pods.
6. Verify that flow records include the UDN network name in
   `SrcK8S_NetworkName` and `DstK8S_NetworkName` fields.
7. Verify the `Udns` field in flow records contains the UDN reference.

## Verify
- Flow records from UDN pods include the UDN network name.
- `SrcK8S_NetworkName` and `DstK8S_NetworkName` fields match the UDN.
- `Udns` field is populated with the UDN identifier.

## Cleanup
Delete the UDN pods and UserDefinedNetwork.
Remove secondary network config from FlowCollector.
Delete the FlowCollector CR.
Delete test namespaces.
