# GoBot Architecture & Security

**Document version:** Season 2 | April 2026
**Component:** GoBot Go service (VPS proxy)
**Copyright:** 2026 Sascha Daemgen, IT and More Systems, Recklinghausen
**License:** AGPL-3.0

For the full system architecture (GoBot + GoKey + GoUNITY), see [SYSTEM-ARCHITECTURE.md](SYSTEM-ARCHITECTURE.md).

---

## Overview

GoBot is the network-facing component of the GoBot system. It runs as a Go service on a VPS, holds all SMP/TLS connections to SimpleX messaging servers, and forwards encrypted blocks to GoKey for processing. GoBot is a **dumb proxy** - it cannot decrypt messages, forge commands, or access private keys.

| Property | Details |
|:---------|:--------|
| Language | Go |
| Deployment | Linux VPS, systemd service |
| SMP connections | Hundreds to thousands (Go goroutines) |
| GoKey connection | Single WSS with mTLS |
| GoUNITY connection | HTTPS REST API |
| Message access | NONE - only encrypted 16 KB blocks |
| Key material | NONE - all keys on GoKey (ESP32) |
| Database | SQLite (metadata only: queue IDs, server addresses) |
| Standalone mode | Optional - GoBot can run without GoKey (lower security) |

---

## 1. What GoBot does

### Primary mode (with GoKey)

```
SMP servers <--TLS--> GoBot <--WSS/mTLS--> GoKey (ESP32)

GoBot:
  1. Subscribes to SMP queues for all group members
  2. Receives encrypted 16 KB blocks
  3. Forwards blocks to GoKey via WSS
  4. Receives signed command results from GoKey
  5. Verifies Ed25519 signature + sequence number
  6. Executes command (SMP protocol: SEND, DEL, etc.)
  7. Receives encrypted response blocks from GoKey
  8. Sends response blocks to correct SMP server
  9. Buffers blocks when GoKey is temporarily offline
  10. Monitors GoKey heartbeat, alerts admin on failure
```

### Standalone mode (without GoKey)

```
SMP servers <--TLS--> GoBot (decrypts locally)

GoBot:
  1. Holds all private keys locally (SQLite + SQLCipher)
  2. Decrypts messages, parses commands, encrypts responses
  3. All crypto happens on the VPS
  4. Lower security (~30-40% of SimpleX guarantees)
  5. Upgrade path: add GoKey later without reconfiguration
```

---

## 2. SMP Frame-Level Client

GoBot does NOT implement a full SimpleX chat client. It operates at the SMP frame level:

| Operation | What GoBot does | What GoBot does NOT do |
|:----------|:---------------|:----------------------|
| TLS connections | Opens and maintains TLS 1.3 to SMP servers | - |
| SMP Subscribe | Sends SUB commands to receive messages | - |
| SMP Send | Wraps encrypted blocks in SEND commands | Encrypt/decrypt message content |
| SMP Ack | Acknowledges received messages | - |
| Double Ratchet | - | All ratchet operations (GoKey does this) |
| NaCl crypto | - | All NaCl encrypt/decrypt (GoKey does this) |
| Key management | - | No private keys (GoKey holds them) |

GoBot knows SMP framing (16 KB blocks, signatures, queue IDs) but not message content.

---

## 3. Security analysis (GoBot specific)

### What a VPS attacker gets

| Access level | What they see |
|:-------------|:-------------|
| Network sniffing | TLS-encrypted blocks (SMP) + TLS-encrypted WSS (GoKey) |
| Root on VPS | Encrypted blocks in transit + signed command strings |
| Database access | Queue IDs, SMP server addresses, sequence counters |
| Process memory | Encrypted blocks, no plaintext, no keys |
| Code modification | Can drop/delay blocks (DoS) but cannot decrypt or forge |

### What a VPS attacker can do (damage potential)

| Attack | Impact | Detection |
|:-------|:-------|:----------|
| Drop all blocks | Bot goes silent | GoKey heartbeat timeout |
| Drop specific blocks | Targeted DoS on conversations | Sequence gap monitoring |
| Delay blocks | Slow moderation response | Timestamp checking |
| Withhold signed commands | Commands not executed | Command ack protocol |
| Replay signed commands | Rejected (sequence number) | Built into protocol |
| Forge commands | Rejected (invalid Ed25519 signature) | Built into protocol |
| Traffic analysis | Sees timing patterns | Constant-size responses from GoKey |

### Hardening measures

| Measure | Purpose |
|:--------|:--------|
| mTLS with certificate pinning | Only THIS GoKey can connect |
| Ed25519 command verification | Cannot forge commands |
| Sequence numbers | Cannot replay commands |
| Command acknowledgments | Detects withheld commands |
| Heartbeat monitoring | Detects GoKey disconnection |
| Block buffering with TTL | Survives temporary GoKey offline |
| Minimal container (no shell) | Reduces post-compromise toolkit |
| iptables egress filtering | Prevents data exfiltration |
| Separate user (gobot:gobot) | Process isolation |

---

## 4. Standalone mode security

When running without GoKey, GoBot holds all keys locally. This mode exists for users who want a quick bot without buying hardware.

| Threat | Standalone | With GoKey |
|:-------|:-----------|:-----------|
| VPS root compromise | Full message access | Only encrypted blocks |
| Key theft | Keys on disk (SQLCipher) | Keys in ESP32 eFuse |
| Message logging | Possible via code modification | Impossible (sealed firmware) |
| Server seizure | Full forensics possible | Only encrypted data |
| Command forgery | Possible with code access | Impossible (Ed25519 on ESP32) |

**Recommendation:** Standalone mode for testing and non-sensitive groups. GoKey mode for any group where privacy matters.

---

## 5. Dependencies

| Dependency | Version | Purpose |
|:-----------|:--------|:--------|
| Go | 1.22+ | Runtime |
| gorilla/websocket | latest | WSS server for GoKey |
| mattn/go-sqlite3 | latest | Metadata storage |
| crypto/ed25519 | stdlib | Command signature verification |
| crypto/tls | stdlib | mTLS for GoKey, TLS for SMP |

---

## 6. Deployment

```bash
# Build
go build -o gobot ./cmd/gobot

# Configure
cp gobot.example.yaml gobot.yaml
# Edit: SMP servers, GoKey certificate paths, admin contact

# Run
./gobot --config gobot.yaml

# Or as systemd service
sudo cp gobot.service /etc/systemd/system/
sudo systemctl enable gobot
sudo systemctl start gobot
```

### systemd service

```ini
[Unit]
Description=GoBot SimpleX Moderation Proxy
After=network.target

[Service]
Type=simple
User=gobot
Group=gobot
ExecStart=/opt/gobot/gobot --config /opt/gobot/gobot.yaml
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

---

## 7. Known vulnerabilities

| ID | Severity | Description | Status |
|:---|:---------|:------------|:-------|
| GB-SEC-01 | HIGH | Standalone mode: all keys on disk | By design - GoKey upgrade resolves |
| GB-SEC-02 | MEDIUM | VPS can selectively drop messages | Mitigated by sequence monitoring |
| GB-SEC-03 | MEDIUM | VPS can withhold signed commands | Mitigated by ack protocol |
| GB-SEC-04 | LOW | Metadata visible (timing, queue IDs) | Mitigated by constant-size responses |

---

*GoBot Architecture & Security v1 - April 2026*
*IT and More Systems, Recklinghausen, Germany*
