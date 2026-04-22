# Test: Verify NodeSelector and Tolerations

## Metadata
- Author: memodi
- Priority: High
- CaseID: 54
- Labels: [Serial]
- Timeout: 20m
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Patch the FlowCollector to add a nodeSelector `kubernetes.io/os: linux` to
   the eBPF agent and FLP processor.
2. Wait for the FlowCollector to reconcile and all pods to restart.
3. Verify that the eBPF agent daemonset pods have the nodeSelector applied.
4. Verify that the FLP deployment pods have the nodeSelector applied.
5. Add a toleration to the FlowCollector eBPF agent configuration and verify
   it propagates to the daemonset pods.

## Verify
- eBPF agent daemonset pod spec contains the configured nodeSelector.
- FLP deployment pod spec contains the configured nodeSelector.
- Tolerations are correctly propagated to the eBPF agent pods.

## Cleanup
Remove the nodeSelector and tolerations from the FlowCollector.
Delete the FlowCollector CR.
