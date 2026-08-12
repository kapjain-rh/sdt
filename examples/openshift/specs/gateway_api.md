# Test: Verify Gateway API Three-Level Owner Metadata

## Metadata
- Author: memodi
- Priority: High
- CaseID: 45
- Labels: [Serial]
- Timeout: 20m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Deploy Gateway API resources (Gateway, HTTPRoute) using template
   `templates/netobserv/gateway-api-template.yaml`.
2. Generate traffic through the Gateway API route.
3. Query Loki for flows associated with the Gateway API pods.
4. Verify that flow records include three-level owner metadata:
   Pod -> ReplicaSet -> Deployment (or Gateway-specific ownership chain).

## Verify
- Flow records from Gateway API pods contain owner hierarchy metadata.
- Resource ownership is correctly tracked through the Gateway API chain.

## Cleanup
Delete the Gateway API resources.
Delete the FlowCollector CR.
Delete test namespaces.
