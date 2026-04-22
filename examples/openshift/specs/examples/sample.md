# Test: Verify Pod Runs Successfully

## Metadata
- Author: sdt-team
- Priority: Critical
- CaseID: 1001
- Labels: [Smoke]
- Timeout: 10m
- Group: with-nginx

## Setup
Ensure the namespace `sdt-example-test` exists and the nginx pod is running.

## Steps
1. Get the nginx pod in namespace `sdt-example-test` and verify its status is Running.
2. Execute `curl -s localhost` inside the nginx pod and verify it returns the default nginx welcome page.
3. Check that the pod has no restart count greater than 0.

## Verify
- The nginx pod should be in Running phase with all containers ready.
- The curl command output should contain "Welcome to nginx".
- Pod restart count should be 0.

## Cleanup
No additional cleanup needed; the group post-test handles nginx removal.
