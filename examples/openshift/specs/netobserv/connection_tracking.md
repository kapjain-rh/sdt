# Test: Verify Connection Tracking

## Metadata
- Author: memodi
- Priority: High
- CaseID: 36
- Labels: [Serial, Slow]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-ping-pods]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy the ping pods using the fixture `test-ping-pods`.
Wait for traffic to be generated between the ping pods.

## Steps
1. Query Loki for flow logs between the ping client and server pods.
2. Verify that connection tracking fields are present in the flow records.
3. Check that TCP connection state transitions are recorded, including
   TCP_RST flags.
4. Verify that conversation tracking fields (conversationId,
   conversationTerminatingTimeout) are populated.

## Verify
- Flow records contain connection tracking metadata.
- TCP flag transitions are captured correctly.
- Conversation IDs are assigned to related flows.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
