# GoBot - Technical Concept
# Hardware-Secured Moderation Bot for SimpleX Groups

**Project:** GoBot System (GoBot + GoKey + GoUNITY)
**Author:** Sascha Daemgen / IT and More Systems
**Date:** 2026-04-04
**Status:** Season 1 Complete, Season 2 Planned

---

## 1. Overview

GoBot is a moderation and verification system for SimpleX Chat groups.
It splits the bot into three components that work together:

**GoBot** is a Go service on a VPS. It holds all SMP/TLS connections
to SimpleX messaging servers. It receives encrypted 16 KB message
blocks but cannot decrypt them. It is a dumb proxy that forwards
encrypted blocks to GoKey and executes signed commands it receives
back. GoBot never sees message content.

**GoKey** is an ESP32-S3 device at the user's home. It holds all
private keys (eFuse + ATECC608B), all ratchet state (encrypted NVS),
and performs all cryptographic operations. It decrypts incoming
blocks (3-4 ms per message), checks for bot commands, and sends
back only the command result - never the message text. Built on
SimpleGo's native C SMP implementation (47 files, 21,863 LOC).

**GoUNITY** is a certificate authority based on a fork of
smallstep/certificates (step-ca). It issues Ed25519 certificates
for user identity verification with challenge-response authentication.
Bans are linked to verified usernames, not SimpleX profiles.

The architecture is modeled after Cloudflare Keyless SSL, Qubes
Split GPG, and banking HSM infrastructure - proven patterns applied
for the first time to E2E encrypted messenger bots.

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

[GoUNITY Server]
Certificate Authority (step-ca fork)
Issues Ed25519 certificates
CRL distribution
Challenge-response verification
HSM-backed signing key (YubiKey)
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

---

## 3. GoBot (VPS proxy)

### What it does

- Holds hundreds to thousands of SMP/TLS connections (Go goroutines)
- Receives encrypted 16 KB blocks from SMP servers
- Forwards blocks to GoKey via single WSS connection (mTLS)
- Receives signed command results from GoKey
- Verifies Ed25519 signature + sequence number + timestamp
- Executes commands via SMP protocol (SEND, DEL, etc.)
- Forwards encrypted response blocks from GoKey to SMP servers
- Buffers blocks when GoKey is temporarily offline
- Monitors GoKey heartbeat, alerts admin on failure

### What it cannot do

- Decrypt any message (no keys)
- Forge commands (no signing key)
- Replay commands (sequence number protection)
- Read message content (not even in RAM)

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
| AES-256-GCM / ChaCha20 decrypt (Layer 1, 16 KB) | ~1.5 ms |
| Zstd decompress | ~0.5 ms |
| JSON parse | ~0.2 ms |
| Ed25519 sign (command result) | ~26 ms |
| **Total per message** | **~3-4 ms** (excluding signing) |

30 messages per minute = one every 2 seconds. GoKey needs 4 ms.
500x faster than needed.

### eFuse security (IRREVERSIBLE after burn)

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

---

## 5. Security findings from independent review

### Response Oracle (CRITICAL - fixed in design)

If GoKey sends "IGNORE" for non-commands and "CMD:kick:user3"
for commands, the VPS learns which messages trigger commands
through size and timing differences. Over weeks, an attacker
builds a profile of who sends bot commands.

**Fix (built into the wire protocol):** Every response is
constant-size, constant-time. Non-commands generate a dummy
16 KB block. Random delay (100-500ms) added to all responses.
The VPS cannot distinguish commands from non-commands.

### Command Replay (CRITICAL - fixed in design)

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

### Verification flow (full certificate variant)

```
1. User registers at id.simplego.dev (email + payment)
2. GoUNITY generates Ed25519 keypair
3. GoUNITY signs certificate with CA key (in YubiKey)
4. User gets: private key + signed certificate

5. User joins GoBot group, sends certificate via DM
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

### CRL synchronization

GoKey fetches CRL daily from id.simplego.dev/v1/crl.
CRL is signed by CA key. GoKey verifies signature.
Stored in encrypted NVS Flash.

---

## 7. Moderation engine

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

## 8. Security levels

```
Level 1: GoBot standalone (no GoKey)
  All keys on VPS. Simple. ~30-40% of SimpleX security.

Level 2: GoBot + GoKey
  VPS is dumb proxy. Keys on ESP32. ~85-90% security.

Level 3: GoBot + GoKey + TEE (AMD SEV-SNP, future)
  Server in encrypted VM. Keys on ESP32. ~95% security.
```

---

## 9. Season plan

| Season | Focus |
|:-------|:------|
| 1 | Research, prototype (TypeScript), API verification, architecture design |
| 2 | GoBot Go service, GoKey Wire Protocol, permission system |
| 3 | GoKey ESP32 firmware (SimpleGo template) |
| 4 | GoUNITY integration (step-ca, certificates, challenge-response) |
| 5 | Auto-moderation, multi-group, admin dashboard |

---

## 10. Comparable architectures

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

*GoBot Technical Concept v3 - April 2026*
*IT and More Systems, Recklinghausen, Germany*
