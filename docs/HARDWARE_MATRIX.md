# Hardware field-report matrix

Where NetGrip runs on real hardware gets recorded here, one row per board
family. Every cell starts untested and is filled only from a field report,
never from the datasheet. A blank cell means "no field data yet", not "does
not work".

| Device | Board / SoC | Arch | OpenWrt | Self-update | HTTPS | Mgmt port | MQTT / HA | Agent | MultiWAN | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| LGS352C switch | RTL9311 | mips (MSB) | vendor init.d + rpcd | yes, overlay-tight | yes | lan27 | untested | yes | n/a | ~20.3 MB overlay; self-update needed the freed-space fix; SIGTERM does not stop it, `kill -9` needed |
| rt3 router | not recorded | arm64 | not recorded | untested | untested | untested | yes (19 HA entities) | yes | untested | full NetGrip + MQTT verified live |
| rt-lab router | not recorded | mipsle | 25.12.5 (apk) | untested | untested | untested | yes | yes | yes (perf fix verified) | apk only, no opkg; no curl, use wget |
| Flint2 / rt2 / rt4 | not recorded | not recorded | not recorded | untested | untested | untested | untested | standalone agent only | untested | run the NetPulse agent, not full NetGrip |

## Feature meanings

- **Self-update** - the daemon replaces its own binary from a release asset and
  comes back on the new version.
- **HTTPS** - the panel serves TLS with a chosen certificate.
- **Mgmt port** - the chassis marks the port the panel is reached through.
- **MQTT / HA** - Home Assistant discovery and telemetry publish to a broker.
- **Agent** - the embedded NetPulse agent pushes telemetry to a NetPulse server.
- **MultiWAN** - mwan3 status is read and reflected in the panel.

## How to add a field report

1. Add (or update) one row for the board family.
2. Fill only what was observed on the device; leave the rest blank.
3. Put the one-line result in Notes: what worked, what needed a workaround,
   and a pointer to the issue or release that fixed it.
