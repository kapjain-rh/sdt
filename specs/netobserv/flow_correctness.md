# Test: Verify Flow Correctness and Metrics

## Metadata
- Author: memodi
- Priority: Critical
- CaseID: 41
- Labels: [Serial]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy test traffic pods using the fixture `test-traffic`.
Wait at least 30 seconds for flow data to be collected and sent to Loki.

## Steps
1. Get an authentication token for querying the Loki API.
2. Query the Loki gateway route for flow logs filtering by
   SrcK8S_Namespace=test-client and DstK8S_Namespace=test-server.
3. Parse the returned flow log records as JSON.
4. Verify each flow record has the required deterministic fields:
   AgentIP matches DstK8S_HostIP, Bytes > 0, TimeFlowEndMs and
   TimeFlowStartMs are recent timestamps, TimeReceived is a valid unix timestamp.
5. Query the Prometheus metric `netobserv_ingest_flows_processed` and verify
   it is greater than 0.
6. Query the Prometheus metric `netobserv_agent_exported_batch_total` and
   verify it is greater than 0.

## Verify
- Flow records are returned from Loki for the test traffic.
- Flow record fields AgentIP, Bytes, TimeFlowEndMs, TimeFlowStartMs are valid.
- The Prometheus metric `netobserv_ingest_flows_processed` > 0.
- The Prometheus metric `netobserv_agent_exported_batch_total` > 0.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
