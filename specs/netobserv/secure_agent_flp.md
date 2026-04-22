# Test: Secure Communications Between Agent and FLP

## Metadata
- Author: memodi
- Priority: High
- CaseID: 60
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Patch the FlowCollector to enable TLS encryption between the eBPF
   agent and FLP processor by setting `processor.service.tlsType=Auto`.
2. Wait for the eBPF agent and FLP pods to restart with TLS configured.
3. Verify that the eBPF agent pods have TLS certificates mounted.
4. Verify that the FLP service is configured with TLS termination.
5. Query Loki for flows and verify that flow collection still works
   over the encrypted channel.

## Verify
- eBPF agent pods have TLS certificate volumes mounted.
- FLP service uses TLS for accepting data from agents.
- Flow collection continues to work correctly over the encrypted channel.
- No TLS-related errors in agent or FLP logs.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
