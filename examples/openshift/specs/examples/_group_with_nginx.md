# Group: with-nginx

## Metadata
- Timeout: 15m

## Pre-Test
Deploy an nginx pod in namespace `sdt-example-test` using the fixture `nginx-server`.
Wait for the nginx pod to be running and ready.

## Post-Test
Delete the nginx pod from namespace `sdt-example-test`.
