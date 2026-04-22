# Group: with-loki

## Metadata
- Timeout: 45m

## Pre-Test 

1. Detect the cluster IP stack type by inspecting network.operator cluster .spec.serviceNetwork — one of ipv4single, ipv6single, or dualstack.
2. Validate the cluster has at least 10Gi available memory and 6 CPUs across schedulable Linux worker nodes. Skip if resources are insufficient. Also skip on AWS STS clusters (where kube-system/aws-creds secret is not found).
3. Check if the Loki operator (loki-operator) is already installed in openshift-operators-redhat by running `oc get subscription loki-operator -n openshift-operators-redhat --ignore-not-found`. If the output is empty, the operator is not installed (this is expected, not an error). IMPORTANT: The Loki Operator MUST be installed in the `openshift-operators-redhat` namespace — do NOT use `openshift-loki-operator` or any other namespace.
4. Query the available channels for loki-operator from the redhat-operators catalog ONLY. IMPORTANT: There are multiple loki-operator packagemanifests (redhat-operators and community-operators). You MUST use the one from redhat-operators. First list all with: `oc get packagemanifest loki-operator -n openshift-marketplace -o jsonpath='{range .items[*]}{.status.catalogSource}={.status.channels[*].name}{"\n"}{end}'` then pick the channels from the line starting with `redhat-operators=`. Select the latest `stable-*` channel (e.g., `stable-6.2`). Do NOT use the `alpha` channel. Do NOT use channels from community-operators catalog. Skip if no stable channel is found.
5. If the Loki operator is not installed, install it: First, create the OperatorGroup in openshift-operators-redhat with AllNamespaces install mode (spec.targetNamespaces should be empty or omitted) using template templates/netobserv/subscription/allnamespace-og.yaml. Then create the Subscription in openshift-operators-redhat using template templates/netobserv/subscription/sub-template.yaml with the discovered stable channel and redhat-operators as the source. Wait for pods with label name=loki-operator-controller-manager to be ready in openshift-operators-redhat. Verify the CSV reaches Succeeded phase in openshift-operators-redhat. If the Loki operator is already installed but on a different channel than the discovered one, uninstall it and reinstall with the correct channel.
6. Get the default StorageClass from the cluster. Skip if none exists.
7. Determine the object storage type based on the cloud platform: AWS → s3, GCP → gcs, Azure → azure, OpenStack → swift. For other platforms, check for ODF (openshift-storage.noobaa.io StorageClass) or MinIO; deploy MinIO in minio-aosqe namespace if neither exists. Skip if no storage type is available (except on IPv6-only clusters which use in-memory storage).
8. Create the LokiStack namespace netobserv-loki using template templates/netobserv/subscription/namespace.yaml.
9. Prepare cloud storage resources for the LokiStack backend: Create the storage bucket/container on the cloud provider (S3 bucket, Azure blob container, GCS bucket, Swift container, ODF ObjectBucketClaim, or MinIO bucket). For workload identity (STS) clusters, create the required IAM roles/managed identities/service accounts and patch the Loki operator subscription with the credentials. Create the objectstore-secret in netobserv-loki with the appropriate storage credentials. The bucket name follows the pattern netobserv-loki-{infrastructure-name}.
10. Deploy the LokiStack CR in netobserv-loki using template templates/netobserv/loki/lokistack-simple.yaml with size 1x.demo, tenant openshift-network, and the storage configuration from previous steps. For IPv6-only clusters, set EnableIPV6=true.
11. Wait for the LokiStack resource to appear, then wait for all components to be ready with a **10-minute timeout** (the ingester StatefulSet uses ordered rollout — ingester-1 only starts after ingester-0 is fully ready, so the full rollout takes several minutes). Components to wait for: Deployments lokistack-distributor, lokistack-gateway, lokistack-querier, lokistack-query-frontend; StatefulSets lokistack-compactor, lokistack-index-gateway, lokistack-ingester (wait for all replicas). Use `oc wait lokistack/lokistack -n netobserv-loki --for=condition=Ready --timeout=600s` or poll until the Ready condition is True. On STS clusters, validate the CredentialsRequest and service accounts (lokistack, lokistack-ruler).
12. Get the LokiStack gateway route URL for flow log queries. The route is named `lokistack` (matching the LokiStack CR name), NOT `lokistack-gateway-http` (which is the backing service name). Query it in the `netobserv-loki` namespace: `oc get route lokistack -n netobserv-loki -o jsonpath='{.spec.host}'`

## Pre-Test Validation
1. The Loki operator subscription exists in openshift-operators-redhat: `oc get subscription loki-operator -n openshift-operators-redhat`
2. The Loki operator CSV is in Succeeded phase in openshift-operators-redhat.
3. Namespace netobserv-loki exists: `oc get namespace netobserv-loki`
4. The LokiStack CR is ready in netobserv-loki: `oc get lokistack lokistack -n netobserv-loki -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'` should return True.
5. The LokiStack gateway route exists in netobserv-loki: `oc get route lokistack -n netobserv-loki`

## Post-Test
 1. Delete the LokiStack CR from netobserv-loki and delete all PVCs with label app.kubernetes.io/instance=lokistack.
 2. Remove the cloud storage resources: Delete the storage bucket/container from the cloud provider. On STS clusters, delete the IAM roles/managed identities/service accounts and remove the credential patch from the Loki operator subscription, then wait for operator pods to restart. Delete the objectstore-secret.
 3. If the Loki operator was installed by the pre-test (was not pre-existing), uninstall the operator subscription and CSV from openshift-operators-redhat. The namespace openshift-operators-redhat is preserved.
 4. Delete the netobserv-loki namespace.                                                                                                                                           
                                           
