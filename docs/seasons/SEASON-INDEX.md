# Season Index

Overview of all development seasons in the GoBot project.

---

## Seasons

| Season | Focus | Status | Timeframe |
|:-------|:------|:-------|:----------|
| 1 | Research, prototype, architecture design | Completed | - April 4, 2026 |
| 2 | GoBot Go service + GoKey Wire Protocol | Active | April 4, 2026 - |
| 3 | GoKey ESP32-S3 firmware | Planned | - |
| 4 | GoUNITY CA integration | Planned | - |
| 5 | Auto-moderation, multi-group, web dashboard | Planned | - |

---

## Season 1 - Research and Architecture

**Goal:** Explore the SimpleX bot ecosystem, build a TypeScript
prototype, design the split-crypto architecture (GoBot + GoKey +
GoUNITY).

**Key Results:**
- TypeScript prototype running on VPS (smp.simplego.dev)
- SimpleX Bot API verified and documented
- Split-crypto architecture designed and validated
- GoUNITY (step-ca fork) repository created
- Security model with Response Oracle, replay prevention,
  ChaCha20 over AES defined
- Comparable architectures identified (Cloudflare Keyless SSL,
  Qubes Split GPG, FIDO2, hardware wallets, Apple PCC)

**Documents:**
- [Season 1 Protocol](SEASON-1-PROTOCOL.md)
- [Season 1 Handoff](SEASON-1-HANDOFF.md)

---

## Season 2 - GoBot Go Service

**Goal:** Build GoBot as a production Go service. Define the
GoKey Wire Protocol. Implement standalone mode for testing.
Prepare WSS server for GoKey connection.

**Sprints:**
- Sprint 0: GoKey Wire Protocol specification
- Sprint 1: Go project setup
- Sprint 2: SMP proxy
- Sprint 3: Standalone mode
- Sprint 4: WSS server for GoKey
- Sprint 5: Documentation + season close

**Documents:**
- [Season Plan](SEASON-PLAN.md)
- [GoKey Wire Protocol](../GOKEY-WIRE-PROTOCOL.md)

---

## Season 3 - GoKey ESP32 Firmware (Planned)

**Goal:** Build the GoKey firmware for ESP32-S3. Implement
ChaCha20-Poly1305 decryption, Ed25519 signing, eFuse key
storage, NVS sequence counter, wire protocol client.

---

## Season 4 - GoUNITY Integration (Planned)

**Goal:** Integrate GoUNITY certificate authority for user
identity verification. Challenge-response flow, CRL
distribution, certificate-linked bans.

---

## Season 5 - Auto-Moderation and Management (Planned)

**Goal:** Content filtering, multi-group management, web
dashboard for operators.

---

*This document is updated at the start and end of each season.*
