# Test: Verify TCP Flags

## Metadata
- Author: memodi
- Priority: High
- CaseID: 62
- Labels: [Disruptive]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Wait for TCP traffic to generate flows in Loki.
2. Query Loki for flows between test-client and test-server.
3. Verify that the `Flags` field in flow records contains expected TCP
   flags such as `SYN`, `ACK`, `FIN`.
4. Deploy a SYN flood test client using template
   `templates/netobserv/test-SYN-flood-client_template.yaml` to generate
   abnormal TCP traffic.
5. Query Loki for flows from the SYN flood client and verify that
   flows show SYN-only flag patterns.

## Verify
- Normal TCP flow records contain standard flag sequences (SYN, ACK, FIN).
- SYN flood traffic generates flows with SYN-only flags.
- TCP flag data is accurately recorded in flow records.

## Cleanup
Delete the SYN flood client pod.
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
