# Test: Pause Network Observability Functions

## Metadata
- Author: kapjain
- Priority: Medium
- CaseID: 88334
- Labels: [Serial]
- Timeout: 20m
- Group: with-loki
- Fixtures: [flowcollector-service-model]

## Setup
1. Deploy the FlowCollector using the fixture `flowcollector-service-model`.
2. Wait for the FlowCollector to reach Ready state.
3. Get all netobserv-managed components before pause (excluding pods with dynamic IDs): `oc get service,deployment,daemonset,serviceaccount,networkpolicy,configmap,secret -A -l netobserv-managed=true -o name`. Save the output for comparison after pause.

## Steps
1. Patch the FlowCollector to pause: `oc patch flowcollector cluster --type=merge -p '{"spec":{"execution":{"mode":"OnHold"}}}'`
2. Wait for FlowCollector status to show "on hold" message by polling `oc get flowcollector cluster -o jsonpath='{.status.conditions[?(@.message=="FlowCollector is on hold")]}'` until it returns a non-empty result (timeout 150 seconds, poll every 5 seconds).
3. Wait for 60 sec to stable 
3. Get all netobserv-managed components after pause: `oc get service,deployment,daemonset,serviceaccount,networkpolicy,configmap,secret -A -l netobserv-managed=true -o name`. Save this output.
4. Verify the following components still exist after pause: deployment.apps/netobserv-plugin-static, service/netobserv-plugin-static, networkpolicy.networking.k8s.io/netobserv, configmap/lokistack-ca-bundle, configmap/lokistack-gateway-ca-bundle, configmap/grafana-dashboard-netobserv-health, configmap/netobserv-main, secret/lokistack-query-frontend-http.
5. Verify netobserv-managed pods after pause: `oc get pod -A -l netobserv-managed=true -o name`. Assert that netobserv-plugin-static pods exist, but flowlogs-pipeline pods, netobserv-ebpf-agent pods, and non-static netobserv-plugin pods are deleted.
6. Verify that all components from the before-pause list that are NOT in the should-remain list (step 4) are actually deleted after pause.
7. Resume the FlowCollector: `oc patch flowcollector cluster --type=merge -p '{"spec":{"execution":{"mode":"Running"}}}'`
8. Wait for FlowCollector to reach Ready state again.
9. Verify no "on hold" message in FlowCollector status: `oc get flowcollector cluster -o jsonpath='{.status.conditions[?(@.message=="FlowCollector is on hold")]}'` should return empty.
10. Wait 60 seconds for flows to be collected and written to Loki after resume and please collect flows after Resume the FlowCollector.


## Cleanup
1. Ensure the FlowCollector execution mode is set back to Running: `oc patch flowcollector cluster --type=merge -p '{"spec":{"execution":{"mode":"Running"}}}' --ignore-not-found`
2. Delete the FlowCollector CR via the fixture cleanup.
