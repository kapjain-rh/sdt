# Test: NetObserv with Sampling 50

## Metadata
- Author: memodi
- Priority: High
- CaseID: 57
- Labels: [Serial, Slow]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-sampling-50, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-sampling-50`
which sets Sampling=50.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Wait for flow data to be collected in Loki.
2. Query Loki for flows between test-client and test-server.
3. Verify that the `Sampling` field in flow records equals 50.
4. Compare the total flow count with a Sampling=1 baseline to verify
   that fewer flows are captured (approximately 1/50th).

## Verify
- Flow records contain `Sampling: 50` field.
- The number of flow records is significantly lower compared to Sampling=1.
- Flow data is still representative of actual network traffic.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
