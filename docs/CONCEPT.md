# GoBot - Technical Concept
# Hardware-Secured Moderation Bot and Community Relay for SimpleX

**Project:** GoBot System (GoBot + GoKey + GoUNITY + GoLab)
**Author:** Sascha Daemgen / IT and More Systems
**Date:** April 16, 2026
**Status:** Season 2 Active

---

## 1. Overview

GoBot is a moderation and verification system for SimpleX Chat
groups, and a community relay engine for the GoLab developer
platform. It splits into components that work together:

**GoBot** is a Go service on a VPS. It holds all SMP/TLS connections
to SimpleX messaging servers. It receives encrypted 16 KB message
blocks but cannot decrypt them. It is a dumb proxy that forwards
encrypted blocks to GoKey and executes signed commands it receives
back. For GoLab, it acts as a relay node that fans out community
messages to channel subscribers. GoBot never sees message content.

**GoKey** is an ESP32-S3 device at the user's home. It holds all
private keys (eFuse + ATECC608B), all ratchet state (encrypted NVS),
and performs all cryptographic operations. It decrypts incoming
blocks (3-4 ms per message), checks for bot commands, and sends
back only the command result - never the message text. GoKey also
serves as a hardware identity anchor for GoLab users. Built on
SimpleGo's native C SMP implementation (47 files, 21,863 LOC).

**GoUNITY** is a certificate authority based on a fork of
smallstep/certificates (step-ca). It issues Ed25519 certificates
for user identity verification with challenge-response authentication.
Bans are linked to verified usernames, not SimpleX profiles. Used
by both SimpleX group moderation and GoLab community management.

**GoLab** is a privacy-first developer community platform that
combines GitLab-style project collaboration with Twitter-style
social feeds. GoLab uses GoBot as its relay engine, GoUNITY for
identity, and optionally GoKey for hardware-backed verification.
All community messages are ActivityStreams 2.0 objects transported
over SMP queues with E2E encryption.

The architecture is modeled after Cloudflare Keyless SSL, Qubes
Split GPG, and banking HSM infrastructure - proven patterns applied
for the first time to E2E encrypted messenger bots and community
platforms.

---

## 2. System architecture

```
[Your VPS]                              [Your home]
GoBot (Go service)                       GoKey (ESP32-S3)
Holds SMP connections                    Holds ALL private keys
Receives encrypted blocks                eFuse sealed firmware
Cannot decrypt anything                  Decrypts, checks commands
  |                                           |
  |--- encrypted 16 KB block ---WSS/mTLS----->|
  |                                           |
  |                                      Decrypt (3-4 ms)
  |                                      Command? -> signed result
  |                                      No command? -> "NOP" (same size)
  |                                           |
  |<-- constant-size response (signed) ------|
  |
  Executes command
  Never saw the message

[GoUNITY Server]                    [GoLab Application]
Certificate Authority               Community Platform
(step-ca fork)                      (Go + TypeScript)
  |                                    |
  Issues Ed25519 certificates          Uses GoBot as relay
  CRL distribution                     Activity streams, projects
  Challenge-response verification      Post persistence, search
  HSM-backed signing key (YubiKey)     Browser client (simplex-js)
```

### Why this split?

Every bot in an E2E encrypted group receives all messages in
cleartext. The bot IS an endpoint. Transport encryption is
irrelevant because the bot decrypts everything.

On a traditional VPS: SSH compromise = full group surveillance.
The attacker copies the private key database and becomes the bot.

With GoBot + GoKey: The VPS has no keys. An attacker who
compromises the server gets encrypted blocks they cannot read,
signed commands they cannot forge, and a database with only
queue IDs and server addresses. The private keys live on an
ESP32-S3 at the user's home with Secure Boot, Flash Encryption,
and permanently disabled JTAG. The only attack is physical
access with laboratory equipment.

This same protection extends to GoLab communities: GoBot relays
community posts, reactions, and follows as encrypted blocks.
In GoKey mode, GoBot cannot read community content any more
than it can read group messages.

---

## 3. GoBot (VPS proxy + community relay)

### What it does

**Moderation role:**
- Holds hundreds to thousands of SMP/TLS connections (Go goroutines)
- Receives encrypted 16 KB blocks from SMP servers
- Forwards blocks to GoKey via single WSS connection (mTLS)
- Receives signed command results from GoKey
- Verifies Ed25519 signature + sequence number + timestamp
- Executes commands via SMP protocol (SEND, DEL, etc.)
- Forwards encrypted response blocks from GoKey to SMP servers
- Buffers blocks when GoKey is temporarily offline
- Monitors GoKey heartbeat, alerts admin on failure

**Community relay role (GoLab):**
- Manages SMP queue pairs for all channel subscribers
- Receives community messages via SMP (posts, reactions, follows)
- Verifies GoUNITY certificates and power levels
- Fans out messages to subscriber queues (O(n) per message)
- Forwards to GoLab application server for persistence
- Enforces CRL (rejects messages from revoked certificates)

### What it cannot do

- Decrypt any message (no keys)
- Forge commands (no signing key)
- Replay commands (sequence number protection)
- Read message content (not even in RAM)
- Correlate users across channels (separate queue pairs)

### Standalone mode (optional, lower security)

GoBot can run without GoKey. In this mode it holds all keys
locally and decrypts on the VPS. Lower security (~30-40% of
SimpleX guarantees) but simpler to deploy. Upgrade to GoKey
at any time without reconfiguration.

---

## 4. GoKey (ESP32-S3 crypto engine)

### FreeRTOS task layout

| Task | Core | Stack | Role |
|:-----|:-----|:------|:-----|
| network_task | Core 0 | 16 KB SRAM | WiFi + WSS connection to VPS |
| gokey_task | Core 1 | 16 KB SRAM | Decrypt, parse, encrypt, sign |
| wifi_manager | Core 0 | 4 KB PSRAM | WiFi management, reconnect |

No display task loaded. Frees ~100 KB RAM (LVGL pool + draw
buffers + task stack).

### Crypto performance (ESP32-S3 at 240 MHz)

| Operation | Duration |
|:----------|:---------|
| NaCl crypto_box_open (Layer 2+3) | ~0.3 ms each |
| ChaCha20-Poly1305 decrypt (Layer 1, 16 KB) | ~1.5 ms |
| Zstd decompress | ~0.5 ms |
| JSON parse | ~0.2 ms |
| Ed25519 sign (command result) | ~26 ms |
| **Total per message** | **~3-4 ms** (excluding signing) |

30 messages per minute = one every 2 seconds. GoKey needs 4 ms.
500x faster than needed.

### eFuse security (irreversible after burn)

```
Secure Boot v2:       RSA-3072 signature check at every boot
Flash Encryption:     AES-256-XTS, key in eFuse, hardware-only
JTAG:                 Permanently disabled
UART Download:        Disabled
Direct Boot:          Disabled
```

### ChaCha20-Poly1305 over AES-GCM

The ESP32-S3 hardware AES accelerator is vulnerable to
side-channel power analysis (confirmed on ESP32-V3/C3/C6).
GoKey uses ChaCha20-Poly1305 in software: 3x faster on
ESP32-S3 (3.29 MB/s vs 1.13 MB/s), naturally constant-time,
immune to power analysis.

### Hardware identity for GoLab

Beyond its primary role as crypto engine, GoKey serves as a
hardware identity anchor. GoLab users can bind their GoUNITY
certificate to a physical device and prove possession through
challenge-response verification. The Ed25519 key lives in
eFuse - it cannot be extracted, copied, or exported.

For full GoKey details, see [GoKey Architecture](https://github.com/saschadaemgen/SimpleGo/blob/main/templates/gokey/docs/ARCHITECTURE_AND_SECURITY.md) and [GoKey Concept](https://github.com/saschadaemgen/SimpleGo/blob/main/templates/gokey/docs/CONCEPT.md).

---

## 5. Security findings from independent review

### Response Oracle (critical - fixed in design)

If GoKey sends "IGNORE" for non-commands and "CMD:kick:user3"
for commands, the VPS learns which messages trigger commands
through size and timing differences. Over weeks, an attacker
builds a profile of who sends bot commands.

**Fix (built into the wire protocol):** Every response is
constant-size, constant-time. Non-commands generate a dummy
16 KB block. Random delay (100-500ms) added to all responses.
The VPS cannot distinguish commands from non-commands.

### Command Replay (critical - fixed in design)

Ed25519 signatures are deterministic. Same command = same
signature. Without freshness, a compromised VPS replays old
kick commands indefinitely.

**Fix (built into the wire protocol):** Every signed command
includes: sequence number (monotonic) + timestamp + group ID
+ hash of triggering block. Each signature is unique and
non-replayable.

### Signed command format

```
SIGN(seq_num || timestamp || group_id || block_hash || command)
```

GoBot verifies: valid signature, sequence > last seen,
timestamp within 30 seconds, group ID matches context.
Replay rejected. Forgery rejected.

---

## 6. GoUNITY certificate verification

### Architecture

GoUNITY is a fork of smallstep/certificates (step-ca).
Production-grade CA in Go. Ed25519 native. HSM support.
Apache-2.0 license.

**What step-ca provides (not building ourselves):**
Certificate signing, CRL, HSM integration (YubiKey),
OIDC login, REST API, database backends, custom OID
extensions, Docker deployment.

**What we build on top:**
Web frontend (id.simplego.dev), account system, payment
integration, challenge-response endpoint, GoKey CRL sync.

### Verification flow

```
1. User registers at id.simplego.dev (email + payment)
2. GoUNITY generates Ed25519 keypair
3. GoUNITY signs certificate with CA key (in YubiKey)
4. User gets: private key + signed certificate

5. User joins GoBot group or GoLab community, sends certificate via DM
6. GoKey verifies CA signature (local, offline)
7. GoKey sends challenge nonce
8. User signs nonce with private key
9. GoKey verifies signature against public key from cert
10. Proof: user is the key holder, sharing impossible
```

### Ban enforcement

Bans linked to verified username, not SimpleX member ID.
Rejoin with new profile -> must re-verify -> banned username
rejected. New certificate -> new registration -> costs money.
Applies to both SimpleX groups and GoLab community channels.

### CRL synchronization

GoKey fetches CRL daily from id.simplego.dev/v1/crl.
CRL is signed by CA key. GoKey verifies signature.
Stored in encrypted NVS Flash.

For full GoUNITY details, see [GoUNITY Architecture](https://github.com/saschadaemgen/GoUNITY/blob/main/docs/ARCHITECTURE_AND_SECURITY.md).

---

## 7. GoLab community platform

GoLab is a privacy-first developer community platform that uses
GoBot as its relay engine. It combines GitLab-style project
collaboration (issues, merge requests, wikis) with Twitter-style
social features (activity feeds, posts, follows, reactions).

### Why GoBot as relay

SimpleX uses client-side fan-out for groups: each member sends
to every other member. This creates O(n^2) connections. GoBot
provides centralized fan-out: sender sends once, GoBot distributes
to all subscribers. This scales to thousands of members.

### Message format

GoLab messages are W3C ActivityStreams 2.0 objects:

| Type | GoLab feature |
|:-----|:-------------|
| Create + Note | Post or comment |
| Announce | Repost |
| Like | Reaction |
| Follow | Subscribe to user or channel |
| Block | Ban (moderator action) |
| Update | Edit post or issue |
| Add | Grant role to member |

Every message is signed with the sender's Ed25519 key from
their GoUNITY certificate. GoBot verifies certificates and
permissions before relaying.

### GoBot's role in GoLab

GoBot acts as the relay and gatekeeper:
1. Receives community messages via SMP
2. Verifies sender certificate and power level
3. Fans out to all channel subscriber queues
4. Forwards to GoLab app server for persistence
5. In GoKey mode: all without seeing content

GoLab is a separate Go service that communicates with GoBot
via internal API. GoBot stays focused on relay and moderation.
GoLab handles application logic (channels, posts, search).

For full GoLab details, see [GoLab Architecture](https://github.com/saschadaemgen/GoLab/blob/main/docs/ARCHITECTURE_AND_SECURITY.md) and [GoLab Concept](https://github.com/saschadaemgen/GoLab/blob/main/docs/CONCEPT.md).

---

## 8. Moderation engine

### Permission system

Every mod command checks the sender's SimpleX role:

```
owner/admin:   all commands
moderator:     kick, warn, mute, reports
member/author: verify, report, rules, mystatus, help
observer:      verify, help
```

Role check uses the GroupMember object included with every
incoming group message (groupRcv direction). Built into the
SimpleX protocol - no additional API call needed.

For GoLab communities, permissions use the power level system
(0-100) enforced via GoUNITY certificates. See [GoLab Architecture](https://github.com/saschadaemgen/GoLab/blob/main/docs/ARCHITECTURE_AND_SECURITY.md#4-permission-system) for details.

### Commands

**Admin:** !kick, !ban, !unban, !mute, !unmute, !warn,
!clearwarn, !banlist, !reports, !mode

**User:** !help, !verify, !report, !mystatus, !rules, !ping

### Auto-moderation

Configurable per group: spam detection (message frequency),
flood protection (messages/hour), new member cooldown
(observer role for N minutes), file/link blocking for
new members.

---

## 9. Security levels

```
Level 1: GoBot standalone (no GoKey)
  All keys on VPS. Simple. ~30-40% of SimpleX security.

Level 2: GoBot + GoKey
  VPS is dumb proxy. Keys on ESP32. ~85-90% security.

Level 3: GoBot + GoKey + TEE (AMD SEV-SNP, future)
  Server in encrypted VM. Keys on ESP32. ~95% security.
```

---

## 10. Season plan

| Season | Focus | Status |
|:-------|:------|:-------|
| 1 | Research, prototype (TypeScript), API verification, architecture design | Complete |
| 2 | GoBot Go service, GoKey Wire Protocol, permission system | Active |
| 3 | GoKey ESP32 firmware (SimpleGo template) | Planned |
| 4 | GoUNITY integration (step-ca, certificates, challenge-response) | Planned |
| 5 | GoLab community relay, channel fan-out, ActivityStreams routing | Planned |
| 6 | Auto-moderation, multi-group, admin dashboard | Future |

---

## 11. Comparable architectures

| System | Pattern |
|:-------|:--------|
| Cloudflare Keyless SSL | Edge proxy + remote key server |
| Qubes Split GPG | Network VM + crypto VM |
| FIDO2/WebAuthn | Browser + hardware authenticator |
| Hardware wallets | Companion app + secure element |
| Apple Private Cloud Compute | OHTTP relay + Secure Enclave |
| Banking HSM | Payment terminal + HSM |
| **GoBot + GoKey** | **VPS proxy + ESP32 crypto engine** |

---

## 12. Related components

| Component | Role | Documentation |
|:----------|:-----|:-------------|
| [GoKey](https://github.com/saschadaemgen/SimpleGo) | Hardware crypto engine | [GoKey Architecture](https://github.com/saschadaemgen/SimpleGo/blob/main/templates/gokey/docs/ARCHITECTURE_AND_SECURITY.md) |
| [GoUNITY](https://github.com/saschadaemgen/GoUNITY) | Certificate authority | [GoUNITY Architecture](https://github.com/saschadaemgen/GoUNITY/blob/main/docs/ARCHITECTURE_AND_SECURITY.md) |
| [GoLab](https://github.com/saschadaemgen/GoLab) | Community platform | [GoLab Architecture](https://github.com/saschadaemgen/GoLab/blob/main/docs/ARCHITECTURE_AND_SECURITY.md) |
| [GoKey Wire Protocol](GOKEY-WIRE-PROTOCOL.md) | GoBot-GoKey communication | [Wire Protocol v0.2.0](GOKEY-WIRE-PROTOCOL.md) |
| [SimpleX Bot API](SIMPLEX-BOT-API-REFERENCE.md) | SimpleX SDK types | [API Reference](SIMPLEX-BOT-API-REFERENCE.md) |

---

*GoBot Technical Concept v2 - April 2026*
*IT and More Systems, Recklinghausen, Germany*
