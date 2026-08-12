# Test: Verify UDN with VMs

## Metadata
- Author: memodi
- Priority: High
- CaseID: 68
- Labels: [Disruptive, Slow]
- Timeout: 45m
- Group: with-vms
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
SecondaryNetworks support enabled.
Ensure KubeVirt HyperConverged is ready (handled by group pre-test).

## Steps
1. Create a UserDefinedNetwork (UDN) in a test namespace.
2. Deploy VMs attached to the UDN using template
   `templates/netobserv/virtualization/test-vm-UDN_template.yaml`.
3. Wait for VMs to be running on the UDN.
4. Generate traffic between UDN-attached VMs.
5. Query Loki for flows from the VM pods.
6. Verify flow records reference the UDN network.

## Verify
- Flow records from UDN-attached VMs contain UDN network metadata.
- VM-to-VM traffic over UDN is correctly captured and enriched.

## Cleanup
Delete the VMs.
Delete the UserDefinedNetwork.
Delete the FlowCollector CR.
Delete test namespaces.
