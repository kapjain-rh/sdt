# Test: Verify Flow Enrichment for VM Secondary Interfaces

## Metadata
- Author: memodi
- Priority: High
- CaseID: 67
- Labels: [Disruptive, Slow]
- Timeout: 45m
- Group: with-vms
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default` with
SecondaryNetworks support enabled.
Ensure KubeVirt HyperConverged is ready (handled by group pre-test).

## Steps
1. Create a Layer2 NetworkAttachmentDefinition using template
   `templates/netobserv/virtualization/layer2-nad.yaml`.
2. Deploy VMs with secondary interfaces using template
   `templates/netobserv/virtualization/test-vm-static-IP_template.yaml`.
3. Wait for VMs to be running and accessible.
4. Generate traffic between VMs over the secondary interface.
5. Query Loki for flows from the VM pods.
6. Verify that flow records include `SrcK8S_NetworkName` and
   `DstK8S_NetworkName` fields for the secondary interface.

## Verify
- Flow records from VM secondary interfaces contain network name metadata.
- `SrcK8S_NetworkName` and `DstK8S_NetworkName` identify the secondary network.
- VM-to-VM traffic over secondary interfaces is correctly captured.

## Cleanup
Delete the VMs.
Delete the NetworkAttachmentDefinition.
Delete the FlowCollector CR.
Delete test namespaces.
