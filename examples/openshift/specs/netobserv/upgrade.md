# Test: NetObserv Upgrade Testing

## Metadata
- Author: memodi
- Priority: Critical
- CaseID: 65
- Labels: [Serial]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.
Wait for flow data to be collected in Loki.

## Steps
1. Query Loki and verify flows are being captured before the upgrade.
2. Record the current operator CSV version.
3. Trigger an operator upgrade by approving the pending install plan or
   changing the subscription channel to the next version.
4. Wait for the new CSV to reach Succeeded phase.
5. Wait for the FlowCollector to reconcile and all pods to restart with
   the new version.
6. Query Loki again and verify flows continue to be captured after the upgrade.

## Verify
- Flows are captured before the upgrade.
- The operator CSV version changes after the upgrade.
- Flows continue to be captured after the upgrade without data loss.
- All NetObserv pods are running the new version.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
