# Group: with-kafka

## Metadata
- Timeout: 45m

## Pre-Test
Deploy the Strimzi Kafka operator in namespace `kafka` from the community catalog.
Wait for the Kafka operator pod to be ready.
Deploy a Kafka cluster with TLS enabled using template
`templates/netobserv/kafka/kafka-tls.yaml` in namespace `kafka`.
Wait for the Kafka cluster to be ready (all broker pods running).
Create a KafkaTopic `network-flows` using template
`templates/netobserv/kafka/kafka-topic.yaml`.
Create a KafkaUser `flp-kafka` with TLS authentication using template
`templates/netobserv/kafka/kafka-user.yaml`.
Wait for the KafkaTopic and KafkaUser to be ready.

## Post-Test
Delete the KafkaUser `flp-kafka` and KafkaTopic `network-flows`.
Delete the Kafka cluster from namespace `kafka`.
Uninstall the Strimzi Kafka operator if it was installed by pre-test.
Delete the `kafka` namespace if it was created.
