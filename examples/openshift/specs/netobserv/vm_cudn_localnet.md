# Test: Validate CUDN with Localnet and VMs

## Metadata
- Author: memodi
- Priority: High
- CaseID: 66
- Labels: [Serial]
- Timeout: 45m
- Group: with-vms
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
SecondaryNetworks support enabled.
Ensure KubeVirt HyperConverged is ready (handled by group pre-test).

## Steps
1. Create a ClusterUserDefinedNetwork (CUDN) with Localnet topology.
2. Deploy VMs attached to the CUDN Localnet using template
   `templates/netobserv/virtualization/test-vm-localnet_template.yaml`.
3. Wait for VMs to be running on the CUDN Localnet network.
4. Generate traffic between VMs on the Localnet network.
5. Query Loki for flows from the VM pods.
6. Verify that flow records correctly reference the CUDN Localnet network.

## Verify
- Flow records from CUDN Localnet VMs contain correct network metadata.
- VM-to-VM traffic over CUDN Localnet is correctly captured.

## Cleanup
Delete the VMs.
Delete the ClusterUserDefinedNetwork.
Delete the FlowCollector CR.
Delete test namespaces.
