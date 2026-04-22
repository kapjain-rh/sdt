# Suite: Example Tests

## Metadata
- Timeout: 30m

## Pre-Suite
Verify the cluster is accessible by running `oc whoami`.
Check that the cluster version is available.

## Pre-Test
Create a test namespace `sdt-example-test` if it does not already exist.

## Post-Test
Delete the test namespace `sdt-example-test` if it exists.

## Post-Suite
Log a summary message indicating all example tests have completed.
