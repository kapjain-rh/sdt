# Test: Verify Large Volume Downloads

## Metadata
- Author: memodi
- Priority: High
- CaseID: 49
- Labels: [Serial]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default, test-traffic-large]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Deploy the large-volume traffic pods using the fixture `test-traffic-large`
which creates an nginx server with large blob files and 5 concurrent
download clients.

## Steps
1. Wait for the large file downloads to complete (at least 2 minutes of
   traffic generation).
2. Query Loki for flow logs between test-client and test-server.
3. Verify that flow records have Bytes values consistent with large downloads.
4. Check that no FLP pods have restarted due to OOM during the high-volume
   data processing.

## Verify
- Flow records show large byte counts from the download traffic.
- FLP pods remain stable with zero restarts during high-volume processing.
- No error logs in FLP pods related to memory or buffer overflow.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
