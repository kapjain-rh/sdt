# Test: Verify SCTP, ICMP, ICMPv6 Traffic

## Metadata
- Author: memodi
- Priority: High
- CaseID: 59
- Labels: [Disruptive]
- Timeout: 30m
- Group: with-loki
- Fixtures: [flowcollector-default]

## Setup
Deploy the FlowCollector using the fixture `flowcollector-default`.
Enable the SCTP kernel module on all worker nodes using
`modprobe sctp` via MachineConfig or debug pods.

## Steps
1. Deploy SCTP server and client pods using templates
   `templates/netobserv/networking/sctpserver.yaml` and
   `templates/netobserv/networking/sctpclient.yaml`.
2. Wait for SCTP traffic to be generated between server and client.
3. Query Loki for flows with Proto=132 (SCTP) and verify SCTP flows
   are captured.
4. Generate ICMP traffic by running `ping` between test pods.
5. Query Loki for flows with Proto=1 (ICMP) and verify they are captured.
6. Generate ICMPv6 traffic by running `ping6` between IPv6-enabled pods.
7. Query Loki for flows with Proto=58 (ICMPv6) and verify IcmpType and
   IcmpCode fields are populated.

## Verify
- SCTP flows (Proto=132) are captured in Loki.
- ICMP flows (Proto=1) are captured in Loki.
- ICMPv6 flows (Proto=58) are captured with IcmpType and IcmpCode fields.

## Cleanup
Delete the SCTP server and client pods.
Delete the FlowCollector CR.
Restore worker nodes (SCTP module stays loaded until reboot).
