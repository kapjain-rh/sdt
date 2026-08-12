# Test: Verify DSCP with NetObserv

## Metadata
- Author: memodi
- Priority: High
- CaseID: 38
- Labels: [Serial]
- Timeout: 20m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Deploy a test client pod using template
   `templates/netobserv/networking/test-client-DSCP.yaml` that sends
   traffic with a specific DSCP marking.
2. Deploy a test nginx server in a separate namespace.
3. Wait for traffic with DSCP markings to be generated.
4. Query Loki for flows between the DSCP client and server.
5. Verify that the `Dscp` field in flow records matches the expected
   DSCP value set by the client.

## Verify
- Flow records contain a non-zero `Dscp` field.
- The DSCP value matches the marking set by the test client.

## Cleanup
Delete the DSCP test client and server pods.
Delete test namespaces.
Delete the FlowCollector CR.
