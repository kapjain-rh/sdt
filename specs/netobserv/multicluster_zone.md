# Test: Verify Cluster ID and Zone in MultiCluster Deployment

## Metadata
- Author: memodi
- Priority: High
- CaseID: 50
- Labels: [Serial]
- Timeout: 25m
- Group: with-loki
- Fixtures: [flowcollector-multicluster, test-traffic]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-multicluster`
with MultiClusterDeployment=true and AddZone=true.
Deploy test traffic pods using the fixture `test-traffic`.

## Steps
1. Wait for flow data to be collected in Loki.
2. Query Loki for flows between test-client and test-server.
3. Verify that the `K8S_ClusterName` field is populated in flow records
   and matches the cluster's infrastructure name.
4. Verify that zone fields (`SrcK8S_Zone`, `DstK8S_Zone`) are populated
   in the flow records (if the cluster has multi-zone nodes).

## Verify
- Flow records contain a non-empty `K8S_ClusterName` field.
- `K8S_ClusterName` matches the cluster's `infrastructure.config.openshift.io/cluster` name.
- Zone fields are populated for nodes that have topology zone labels.

## Cleanup
Delete the FlowCollector CR.
Delete namespaces test-server and test-client.
