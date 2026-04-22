# Test: Verify NetObserv Alerts

## Metadata
- Author: memodi
- Priority: High
- CaseID: 35
- Labels: [Serial, Slow]
- Timeout: 30m
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Wait for FlowCollector to be Ready.

## Steps
1. Verify that a PrometheusRule named `netobserv-rules` exists in namespace
   `openshift-netobserv`.
2. Check that the PrometheusRule contains alert rules for `NetObservNoFlows`,
   `NetObservLokiError`, and `NetObservDroppedFlows`.
3. Patch the FlowCollector to use an incorrect Loki URL to trigger the
   `NetObservLokiError` alert.
4. Wait up to 10 minutes for the `NetObservLokiError` alert to become active
   in the AlertManager.
5. Restore the correct Loki URL in the FlowCollector.
6. Verify the FLP and eBPF alert rules can be individually disabled via
   `processor.metrics.disableAlerts`.

## Verify
- PrometheusRule `netobserv-rules` exists and contains the expected alert definitions.
- The `NetObservLokiError` alert fires when Loki is unreachable.
- Disabling alert rules via FlowCollector spec removes them from the PrometheusRule.

## Cleanup
Restore the FlowCollector to its original configuration.
Delete the FlowCollector CR.
