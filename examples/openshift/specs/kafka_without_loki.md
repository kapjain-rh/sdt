# Test: Verify Network Flows Export with Kafka without Loki

## Metadata
- Author: memodi
- Priority: High
- CaseID: 48
- Labels: [Serial]
- Timeout: 25m
- Group: with-kafka
- Fixtures: [flowcollector-no-loki, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-no-loki` which
uses Kafka deployment model with Loki disabled.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Verify that the FlowCollector is configured with Kafka deployment model
   and Loki is disabled (loki.enable=false).
2. Verify that `flowlogs-pipeline-transformer` pods are running.
3. Wait for flow data to be exported to the Kafka topic.
4. Consume messages from the Kafka topic `network-flows` and verify
   flow data is present.
5. Verify that Prometheus metrics are still available even without Loki:
   query `netobserv_ingest_flows_processed` > 0.

## Verify
- FlowCollector operates correctly without Loki backend.
- Kafka topic `network-flows` contains flow data.
- Prometheus metrics are collected regardless of Loki being disabled.
- No error logs in FLP pods related to missing Loki configuration.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
