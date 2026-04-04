---
title: "GoBot Architecture & Security"
sidebar_position: 1
---

# GoBot Architecture & Security

**Document version:** Season 1 | April 2026
**Hardware Model:** ESP32-S3 via SimpleGo firmware (LilyGo T-Deck Plus or custom PCB)
**Server Model:** Node.js + simplex-chat SDK (native FFI)
**Copyright:** 2026 Sascha Daemgen, IT and More Systems, Recklinghausen
**License:** AGPL-3.0

---

## Overview

| Property | Hardware Model | Server Model |
|----------|---------------|-------------|
| Platform | ESP32-S3 Dual-Core 240 MHz, 8 MB PSRAM | Any Linux server, Node.js v22+ |
| SMP Implementation | SimpleGo native C (47 files, 21,863 LOC) | simplex-chat npm (Haskell FFI) |
| Encryption | 4 layers: Double Ratchet (X448) + 2x NaCl + TLS 1.3 | 4 layers via Haskell core |
| Tamper protection | Secure Boot v2 + Flash Encrypt + eFuse + Secure Element | OS-level only (SSH, firewall) |
| Attack surface | Physical access with lab equipment | SSH, OS vulnerabilities, passwords |
| Message storage | None (RAM only, overwritten after processing) | chat.db (SQLCipher encrypted) |
| Bot state storage | NVS Flash (encrypted, ~88 KB usable) | SQLite (unlimited) |
| Status | Design phase (SimpleGo SMP stack proven) | v0.0.1-alpha deployed and running |

---

## 1. Hardware Model: FreeRTOS Task Architecture

GoBot integrates into the existing SimpleGo firmware as an additional FreeRTOS task. The SMP protocol stack, encryption, and network connectivity are already implemented and proven with 7 simultaneous contacts stable.

### Task layout

| Task | Core | Stack | Responsibility |
|------|------|-------|---------------|
| `network_task` | Core 0 | 16 KB SRAM | All TLS connections. Reads SMP frames from server, writes commands. Isolated so a hanging TLS handshake never blocks bot logic. |
| `smp_app_task` | Core 1 | 16 KB SRAM | SMP protocol state machine, ratchet encryption, NVS persistence, contact management. Must run in internal SRAM (PSRAM cache disabled during NVS flash writes). |
| `gobot_task` | Core 1 | 8 KB SRAM | Command parsing, moderation logic, GoUNITY verification, ban enforcement, rate limiting. Reads from rx_ring_buffer, writes to tx_ring_buffer. |
| `wifi_manager` | Core 0 | 4 KB PSRAM | WiFi connection management, multi-network storage, reconnects. |

**Note:** The `lvgl_task` (display rendering) is NOT loaded in bot mode. This frees approximately 100 KB of RAM:
- 64 KB LVGL memory pool
- 25.6 KB DMA draw buffers
- 8 KB LVGL task stack
- Associated PSRAM display cache

### Inter-task communication

| Mechanism | Direction | Description |
|-----------|-----------|-------------|
| `rx_ring_buffer` | Network -> App -> GoBot | Received SMP frames. GoBot reads decrypted messages after smp_app_task processes them. Capacity: 4 frames x 16 KB = 64 KB in PSRAM. |
| `tx_ring_buffer` | GoBot -> App -> Network | Bot responses. GoBot writes reply text, smp_app_task encrypts and network_task sends. |
| `gobot_event_queue` | App -> GoBot | FreeRTOS queue for group events: member join, member leave, role change. GoBot processes sequentially. |
| NVS Flash | GoBot <-> persistent | Ban lists, warnings, verified users, group config. Encrypted when Flash Encryption is enabled. |

### Memory budget (bot mode, no display)

| Region | Total | Used by SMP stack | Available for GoBot | GoBot allocation |
|--------|-------|-------------------|---------------------|-----------------|
| Internal SRAM | 512 KB | ~180 KB | ~330 KB | ~50 KB (task stack + processing buffers) |
| PSRAM | 8 MB | ~200 KB (ratchets + ring buffers) | ~7.8 MB | ~200 KB (ban DB cache, message buffer) |
| NVS Flash | 128 KB | ~40 KB (keys, queues) | ~88 KB | ~40 KB (bans, warnings, verified users) |

---

## 2. Server Model: Node.js Architecture

### Process structure

```
Node.js Process (single-threaded event loop)
  |
  +-- simplex-chat Haskell core (native FFI, embedded)
  |     |-- SMP protocol + encryption
  |     |-- SQLite databases (chat.db + agent.db)
  |     |-- Contact and group management
  |
  +-- GoBot TypeScript application
        |-- Command registry + dispatcher
        |-- Moderation engine
        |-- Rate limiter (sliding window per user)
        |-- GoUNITY verifier (Season 2)
        |-- Event handlers (onMessage, receivedGroupInvitation, etc.)
```

### Key dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `simplex-chat` | v6.5.0-beta.4.4 | SimpleX Chat core with native FFI |
| `@simplex-chat/types` | v0.3.0 | TypeScript type definitions |
| Node.js | v22.22.2 | Runtime |
| TypeScript | ^5.x | Compilation |

### Database files

| File | Content | Encryption |
|------|---------|-----------|
| `nwbot_chat.db` | Messages, contacts, groups | SQLCipher (key in config) |
| `nwbot_agent.db` | SMP queue state, ratchet keys | SQLCipher (key in config) |

**WARNING:** Deleting these files destroys the bot's identity, address, all contacts, and all group memberships permanently. The bot address is derived from the database. New database = new identity.

---

## 3. Four encryption layers (both models)

Every message GoBot sends or receives passes through four cryptographically independent layers. These are identical to standard SimpleX encryption - GoBot is indistinguishable from a regular client on the network.

| # | Layer | Algorithm | Library (HW) | Library (Server) |
|---|-------|-----------|-------------|-----------------|
| 1 | Double Ratchet (E2E) | X3DH (X448) + AES-256-GCM | wolfSSL + mbedTLS | Haskell cryptonite |
| 2 | Per-Queue NaCl | X25519 + XSalsa20 + Poly1305 | libsodium | Haskell NaCl |
| 3 | Server-to-Recipient NaCl | X25519 + XSalsa20 + Poly1305 | libsodium | Haskell NaCl |
| 4 | TLS 1.3 transport | TLS 1.3, ALPN `smp/1` | mbedTLS | Haskell TLS |

Content padding: All messages padded to 16 KB blocks. Message length not observable by network attacker.

---

## 4. Security analysis

### 4.1 The bot security paradox

**Core problem:** Any bot with admin rights in an E2E encrypted group receives all messages in cleartext. This is not a vulnerability - it is an inherent property of E2E encryption where the bot is an endpoint. The four encryption layers protect the transport, but the bot must decrypt to function.

**Impact on SimpleX's security model:** Without a bot, SimpleX groups are protected against everyone - no server, no relay, no third party can read messages. With a bot, security reduces to the security of the bot's execution environment. This is true for every messenger, but SimpleX makes it especially painful because the bot is the ONLY weak point.

### 4.2 Hardware Model security

**Protected against:**

| Threat | Protection mechanism |
|--------|---------------------|
| Remote code execution | No SSH, no shell, no network services |
| Firmware modification | Secure Boot v2 (RSA-3072 signature check at every boot) |
| Flash readout (SPI) | Flash Encryption (AES-256-XTS, key in eFuse) |
| Debug/JTAG access | Permanently disabled via eFuse burn |
| UART download mode | Disabled via eFuse |
| Code injection at boot | Secure Boot verifies bootloader -> bootloader verifies app |
| Message logging | No logging code in sealed firmware (reproducible build verifiable) |
| Network exfiltration | Bot connects only to configured SMP server, no general internet |
| Key extraction (software) | Identity keys in ATECC608B secure element, never in main memory |

**NOT protected against (physical attacks):**

| Threat | Difficulty | Mitigation |
|--------|-----------|-----------|
| Side-channel power analysis | Medium ($100 equipment, ~60K measurements on older ESP32 variants) | ATECC608B for critical keys, but ESP32-S3 AES accelerator may be vulnerable |
| Voltage glitching during boot | Medium-High | Redundant verification in ROM, but not fully immune |
| Electromagnetic fault injection | High | Epoxy potting, conductive mesh overlay |
| Die decapping + microprobing | Very High (lab equipment, destructive) | Device is destroyed, keys in separate secure element |

**eFuse burn sequence (production, IRREVERSIBLE):**

```bash
# Generate signing key (keep offline and backed up!)
espsecure.py generate_signing_key --version 2 secure_boot_key.pem

# Burn Secure Boot key digest
espefuse.py burn_key BLOCK_KEY0 secure_boot_key_digest.bin SECURE_BOOT_DIGEST0

# Enable Secure Boot
espefuse.py burn_efuse SECURE_BOOT_EN 1

# Enable Flash Encryption (permanently)
espefuse.py burn_efuse SPI_BOOT_CRYPT_CNT 7

# Disable all debug interfaces
espefuse.py burn_efuse DIS_USB_JTAG 1
espefuse.py burn_efuse DIS_PAD_JTAG 1
espefuse.py burn_efuse SOFT_DIS_JTAG 7
espefuse.py burn_efuse DIS_DIRECT_BOOT 1
espefuse.py burn_efuse DIS_DOWNLOAD_MANUAL_ENCRYPT 1

# Enable aggressive key revocation
espefuse.py burn_efuse SECURE_BOOT_AGGRESSIVE_REVOKE 1
```

**After this sequence:** The device will only boot firmware signed with the corresponding private key. Flash contents are encrypted. No debug access is possible. The ONLY way to change the firmware is to sign a new image with the original key and flash it via the standard update mechanism. If the signing key is lost, the device is permanently bricked.

### 4.3 Server Model security

**Protected against:**

| Threat | Protection mechanism |
|--------|---------------------|
| Unauthorized access | SSH key-only auth, no root login, firewall |
| Message interception in transit | 4-layer SMP encryption (same as any SimpleX client) |
| Database theft (offline) | SQLCipher encryption on chat.db and agent.db |

**NOT protected against:**

| Threat | Why |
|--------|-----|
| SSH compromise (key theft, OS vuln) | Full access to running process, RAM, code |
| Process memory dump | Bot holds decrypted messages in Node.js heap |
| Code modification | Attacker with root can modify bot code, inject logging |
| Database key extraction | SQLCipher key is in config file or process memory |
| Network exfiltration | Attacker can add forwarding code to send messages externally |
| Hosting provider access | Cloud/VPS provider has physical access to server |

### 4.4 Security rating

| Model | Effective security | Threat model |
|-------|-------------------|-------------|
| Commercial hosted bot (Rose, Combot) | ~15-20% of SimpleX's design | Trust the bot operator + their hosting |
| Self-hosted VPS (GoBot Server) | ~30-40% of SimpleX's design | Trust your server security |
| GoBot Hardware (ESP32-S3 + eFuse) | ~85-90% of SimpleX's design | Only physical lab attacks remain |
| No bot (pure SimpleX) | 100% of SimpleX's design | Only endpoint device compromise |

The missing 10-15% in the Hardware Model: the bot must decrypt messages to process commands (fundamental limitation), and ESP32-S3 is not a certified secure element (physical attacks theoretically possible).

---

## 5. GoUNITY verification security

### 5.1 Certificate system design

| Property | Status |
|----------|--------|
| Algorithm | Ed25519 (fast, small signatures, proven secure) |
| Offline verification | Yes - GoBot verifies locally, no server contact |
| Certificate sharing | VULNERABILITY - mitigated by challenge-response (Season 3+) |
| Replay attacks | VULNERABILITY - mitigated by DM-only + single-use tokens |
| SimpleX identity binding | NONE - certificate proves username, not SimpleX identity |
| Ban evasion cost | New registration + verification fee (5-30 EUR) |
| CRL sync | Daily HTTPS fetch, GoUNITY-signed, verified locally |
| CRL timing window | Up to 24h between revocation and enforcement |

### 5.2 Season 2 pragmatic approach

One-time verification codes eliminate sharing and replay:
1. User gets code from GoUNITY website
2. Sends to GoBot via DM (never in group)
3. GoBot validates via single HTTPS call
4. Code consumed (cannot be reused)
5. Trade-off: GoUNITY must be online during verification

---

## 6. SimpleX API - verified types and methods

Full reference in `docs/SIMPLEX-BOT-API-REFERENCE.md`. Critical types:

### GroupMember (what SimpleX tells us about each user)

```typescript
interface GroupMember {
  groupMemberId: number      // Unique per group
  memberId: string           // Internal ID per connection
  memberRole: GroupMemberRole // owner|admin|moderator|member|author|observer
  memberContactId?: number   // STABLE across groups (if bot contact)
  localDisplayName: string   // NOT STABLE (user can change)
  memberProfile: LocalProfile
  blockedByAdmin: boolean    // Shadow-blocked
}
```

**Identity limitations:** memberIds are connection-scoped. New profile = new IDs. memberContactId is the only cross-group stable identifier (if user is also a bot contact). This is why GoUNITY exists - SimpleX has no persistent person identity.

### Verified API methods

| Method | Tested | Used for |
|--------|--------|---------|
| apiSendTextReply | YES | Bot responses |
| apiListMembers | YES | Permission checks, member info |
| apiJoinGroup | YES | Auto-join on invitation |
| apiUpdateProfile | YES | Avatar, bot indicator |
| apiRemoveMembers | NO | Kick/ban enforcement |
| apiBlockMembersForAll | NO | Shadow blocking |
| apiMembersRole | NO | Role management |
| apiAcceptMember | NO | Doorman approval flow |

---

## 7. Known vulnerabilities - complete list

Honest inventory. No finding downplayed.

### Hardware Model

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| HW-SEC-01 | HIGH | ESP32-S3 hardware AES accelerator may be vulnerable to side-channel power analysis (confirmed on ESP32-V3/C3/C6, unconfirmed on S3) | OPEN - mitigated by ATECC608B for critical keys |
| HW-SEC-02 | HIGH | Messages in PSRAM during processing are theoretically readable via physical attack | OPEN - inherent to any processing device |
| HW-SEC-03 | MEDIUM | Voltage glitching during boot could bypass Secure Boot (theoretical, not demonstrated on S3) | MITIGATED - ROM has redundant checks |
| HW-SEC-04 | LOW | No runtime attestation (ESP32-S3 lacks TEE, unlike ESP32-C6) | DEFERRED - boot-time attestation via Secure Boot is sufficient for threat model |

### Server Model

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| SRV-SEC-01 | CRITICAL | No permission checking on mod commands - any user can !kick | OPEN - Season 2 Priority 1 |
| SRV-SEC-02 | HIGH | Warnings stored in RAM only - lost on restart | OPEN - Season 2 Priority 2 |
| SRV-SEC-03 | HIGH | No ban persistence - kicked users rejoin immediately | OPEN - Season 2 Priority 2 |
| SRV-SEC-04 | HIGH | Bot database encryption key is empty string (dbKey: "") | OPEN - must set proper key |
| SRV-SEC-05 | MEDIUM | Avatar loading fragile - config overwrites during development | OPEN - minor |

### GoUNITY (design phase)

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| GU-SEC-01 | HIGH | Certificate sharing - Alice can give cert to Bob | MITIGATED in Season 2 (one-time codes), RESOLVED in Season 3+ (challenge-response) |
| GU-SEC-02 | HIGH | Replay attacks - cert visible in group can be copied | MITIGATED - DM-only verification + single-use tokens |
| GU-SEC-03 | MEDIUM | No binding between certificate and SimpleX identity | DEFERRED - requires challenge-response |
| GU-SEC-04 | LOW | CRL timing window up to 24h | ACCEPTED - sufficient for non-realtime threat model |

---

## 8. Roadmap

### Season 2 (next)

| Task | Priority | Model |
|------|----------|-------|
| Permission system (memberRole checks) | Critical | Server |
| Persistent storage (SQLite for warnings, bans) | Critical | Server |
| GoUNITY web service + /verify command | High | Server |
| Doorman flow (welcome + verify on join) | High | Server |
| File rename (hilfe->help, zeit->time, datum->date) | Medium | Server |
| Create gounity/ folder in repo | Medium | Both |
| Test apiRemoveMembers, apiBlockMembersForAll | Medium | Server |

### Season 3+

| Task | Model |
|------|-------|
| GoBot as FreeRTOS task (gobot_task integration) | Hardware |
| NVS-based ban/warning storage | Hardware |
| eFuse provisioning documentation | Hardware |
| Reproducible build pipeline | Hardware |
| Challenge-response verification | Both |
| Auto-moderation (spam, flood, cooldown) | Both |
| Multi-group management | Both |
| Web dashboard | Server |

### Hardware roadmap (from SimpleGo)

- **ESP32-P4 board (ordered):** 400 MHz RISC-V dual-core, 32 MB PSRAM, WiFi 6
- **Custom PCB Model 3 Vault:** Triple-vendor secure elements (ATECC608B + OPTIGA + SE050)
- **Three physical kill switches:** WiFi/BLE, LoRa, LTE
- **M.2 slot** for optional LTE modules

---

*GoBot Architecture & Security v1 - April 2026*
*IT and More Systems, Recklinghausen, Germany*
*First hardware-secured moderation bot for encrypted messaging*
