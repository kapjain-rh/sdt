# Group: with-vms

## Metadata
- Timeout: 45m

## Pre-Test
Check if the cluster is BareMetal or has metal worker nodes.
Skip all tests in this group if no baremetal or metal workers are available.
Deploy the KubeVirt HyperConverged operator using template
`templates/netobserv/virtualization/kubevirt-hyperconverged.yaml`.
Wait for KubeVirt HyperConverged to be ready and all virtualization
components to be running.

## Post-Test
Delete the KubeVirt HyperConverged resource.
Clean up any virtualization-related resources.
