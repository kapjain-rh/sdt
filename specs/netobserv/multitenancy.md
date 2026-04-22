# Test: Verify Multi-Tenancy

## Metadata
- Author: memodi
- Priority: High
- CaseID: 51
- Labels: [Disruptive, Slow]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Create two htpasswd users: `user-a` with reader access to the `network`
   Loki tenant, and `user-b` with no Loki access.
2. Add a ClusterRoleBinding granting `user-a` the `netobserv-reader` role.
3. As `user-a`, query the Loki API for flows from the test-client namespace.
   Verify that flows are returned.
4. As `user-b`, query the Loki API for the same flows. Verify that the
   request is denied or returns no results due to RBAC.
5. Remove `user-a`'s reader access and verify they can no longer query flows.

## Verify
- `user-a` with reader role can query Loki and see flow logs.
- `user-b` without reader role cannot access Loki flow data.
- Removing reader access immediately revokes query capability.

## Cleanup
Remove htpasswd users `user-a` and `user-b`.
Remove the ClusterRoleBinding for `user-a`.
Wait for the ClusterOperator `authentication` to stabilize.
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
