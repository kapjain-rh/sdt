# Test: Verify Network Flows Captured with Kafka with TLS

## Metadata
- Author: memodi
- Priority: High
- CaseID: 47
- Labels: [Serial, Slow]
- Timeout: 30m
- Group: with-kafka
- Fixtures: [flowcollector-kafka, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-kafka` which uses
DeploymentModel=Kafka with TLS-enabled Kafka cluster.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Verify that the FlowCollector is configured with Kafka deployment model
   and TLS is enabled for the Kafka connection.
2. Verify that `flowlogs-pipeline-transformer` pods are running in
   `openshift-netobserv` (Kafka mode uses transformer instead of direct FLP).
3. Wait for flow data to flow through the Kafka pipeline to Loki.
4. Query Loki for flows between test-client and test-server.
5. Verify that flow records are present and contain the expected fields.
6. Consume messages from the Kafka topic `network-flows` and verify
   messages are being produced by the eBPF agent.

## Verify
- FlowCollector is in Ready state with Kafka deployment model.
- Flowlogs-pipeline-transformer pods are running and processing Kafka messages.
- Flow records are stored in Loki via the Kafka pipeline.
- Kafka topic `network-flows` contains flow messages.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
