# Test: Verify FlowCollector Service Deployment Model

## Metadata
- Author: memodi
- Priority: High
- CaseID: 61
- Labels: [Serial, Slow]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-service-model, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-service-model`
which uses DeploymentModel=Service.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Verify that the FLP component is deployed as a Kubernetes Service
   instead of a DaemonSet in `openshift-netobserv`.
2. Verify that the eBPF agent pods send data to the FLP service endpoint.
3. Wait for flow data to be collected in Loki.
4. Query Loki for flows between test-client and test-server.
5. Verify flows are captured correctly with the Service deployment model.

## Verify
- FLP runs as a Deployment with a ClusterIP Service (not a DaemonSet).
- eBPF agent pods connect to the FLP service endpoint.
- Flow records are captured and stored in Loki correctly.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
