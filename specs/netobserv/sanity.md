# Test: Sanity Test NetObserv

## Metadata
- Author: memodi
- Priority: Critical
- CaseID: 58
- Labels: [Serial]
- Timeout: 15m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Verify all pods in `netobserv` namespace are Running and Ready.
2. Verify all pods in `netobserv-privileged` namespace are Running and Ready.
3. Verify FlowCollector status condition shows Ready.
4. Check that the console plugin `netobserv-plugin` deployment is available.

## Verify
- All FLP pods are Running with all containers ready.
- All eBPF agent pods are Running with all containers ready.
- FlowCollector CR status is Ready.
- Console plugin is deployed and available.

## Cleanup
Delete the FlowCollector CR and wait for all pods to terminate.
