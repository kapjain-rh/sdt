# Suite: Network Observability Tests

## Metadata
- Timeout: 60m

## Pre-Suite
1. Check if the E2E_RUN_TAGS environment variable contains disconnected. If yes, skip the entire suite.
2. Run oc whoami --show-server to verify the cluster API server is reachable.
3. Save the current kubeconfig context via oc config current-context (restored after any login operations).
4. Check the QE_KUBEADMIN_PASSWORD environment variable. If empty, skip the suite. Otherwise, log in as kubeadmin and obtain a token via oc whoami -t. Assert the token is not empty.
5. Detect if the cluster is Hypershift-hosted. This determines the operator channel: latest for Hypershift, stable otherwise.
6. Create namespace openshift-netobserv-operator using the namespace template.
7. Deploy the catalog source netobserv-konflux-fbc in openshift-netobserv-operator using template templates/netobserv/subscription/catalog-source.yaml. Use the image from MULTISTAGE_PARAM_OVERRIDE_NETOBSERV_CS_IMAGE env var if set, OR quay.io/netobserv/network-observability-operator-catalog:v0.0.0-sha-main for Hypershift clusters (with channel latest), OR the default y-stream catalog image from the template. Wait for the catalog source pod to be ready. For non-Hypershift clusters, apply the ImageDigestMirrorSet using template templates/netobserv/subscription/image-digest-mirror-set.yaml.
8. Check if the netobserv-operator subscription already exists in openshift-netobserv-operator: `oc get subscription netobserv-operator -n openshift-netobserv-operator --ignore-not-found`. If the output is empty, the operator is not installed (this is expected, not an error) — proceed with steps 9-13. If it already exists, skip steps 9-13.
9. Create the OperatorGroup in openshift-netobserv-operator with AllNamespaces install mode using template templates/netobserv/subscription/allnamespace-og.yaml (this template has NO targetNamespaces in spec, which means AllNamespaces mode). IMPORTANT: Do NOT add targetNamespaces to the OperatorGroup spec — the NetObserv operator only supports AllNamespaces install mode. Verify it was created: `oc get operatorgroup -n openshift-netobserv-operator`
10. Create the OLM Subscription in openshift-netobserv-operator using template templates/netobserv/subscription/sub-template.yaml with parameters: name=netobserv-operator, namespace=openshift-netobserv-operator, source=netobserv-konflux-fbc, sourceNamespace=openshift-netobserv-operator, channel=the channel selected in step 5. Verify it was created: `oc get subscription netobserv-operator -n openshift-netobserv-operator`
11. Wait for the operator pod to be ready using exactly this label selector: `app=netobserv-operator` in namespace openshift-netobserv-operator (timeout 5 minutes). Do NOT use `app=netobserv-controller-manager` or any other label — the correct label is `app=netobserv-operator`.
12. Get the actual CSV name from the subscription status: `oc get subscription netobserv-operator -n openshift-netobserv-operator -o jsonpath='{.status.currentCSV}'`. The CSV is named `network-observability-operator.vX.Y.Z` (NOT `netobserv-operator.vX.Y.Z`).
13. Verify that CSV reaches Succeeded phase using the name obtained in step 12 (timeout 5 minutes).
14. Confirm the FlowCollector CRD exists on the cluster.
15. If the upstream catalog was deployed (Hypershift), patch the CSV to enable upstream features.

## Pre-Suite Validation
1. Namespace openshift-netobserv-operator exists: `oc get namespace openshift-netobserv-operator`
2. The netobserv-operator subscription exists and has a currentCSV: `oc get subscription netobserv-operator -n openshift-netobserv-operator -o jsonpath='{.status.currentCSV}'`
3. The operator CSV is in Succeeded phase in openshift-netobserv-operator.
4. The FlowCollector CRD exists on the cluster: `oc get crd flowcollectors.flows.netobserv.io`

## Pre-Test
1. Verify Network Observability Operator Installed in Pre-Suite

## Post-Test
1. Delete the operator subscription and CSV from openshift-netobserv-operator. 
2. The namespace itself should be preserved after test.
2. Remove the netobserv-konflux-fbc catalog source from openshift-netobserv-operator (NOT openshift-marketplace) and the ImageDigestMirrorSet if they were created.

## Post-Suite
1. Collect must-gather logs if any test failed.
2. Delete the openshift-netobserv-operator namespace if it was created by the suite.
