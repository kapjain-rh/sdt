# Test: Verify NetObserv Must-Gather Plugin

## Metadata
- Author: memodi
- Priority: High
- CaseID: 52
- Labels: [Serial]
- Timeout: 15m
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Run `oc adm must-gather --image=quay.io/netobserv/must-gather` to collect
   NetObserv diagnostic data.
2. Verify the must-gather output contains pod logs from the `openshift-netobserv`
   namespace.
3. Verify the must-gather output contains FlowCollector CR definition.
4. Verify the must-gather output contains eBPF agent daemonset information from
   `openshift-netobserv-privileged` namespace.

## Verify
- Must-gather completes successfully without errors.
- Output directory contains logs from FLP pods.
- Output directory contains FlowCollector CR YAML.
- Output directory contains eBPF agent pod data.

## Cleanup
Remove the must-gather output directory.
Delete the FlowCollector CR.
