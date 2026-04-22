# Test: Verify TLS Tracking Feature

## Metadata
- Author: memodi
- Priority: High
- CaseID: 63
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.

## Steps
1. Deploy TLS server and client pods using templates
   `templates/netobserv/test-tls-server_template.yaml` and
   `templates/netobserv/test-tls-client_template.yaml`.
2. Wait for TLS-encrypted traffic to be generated between the pods.
3. Query Loki for flows between the TLS client and server.
4. Verify that TLS tracking fields are populated: `TLSVersion`,
   `TLSTypes`, `TLSCurve`, `TLSCipherSuite`.

## Verify
- Flow records contain a valid `TLSVersion` (e.g., "1.3", "1.2").
- `TLSTypes` field is populated.
- `TLSCurve` field identifies the elliptic curve used.
- `TLSCipherSuite` field identifies the cipher suite negotiated.

## Cleanup
Delete the TLS server and client pods.
Delete test namespaces.
Delete the FlowCollector CR.
