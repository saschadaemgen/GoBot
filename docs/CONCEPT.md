# GoBot - Technical Concept
# Moderation and Automation Bot for SimpleX Groups

**Project:** GoBot - SimpleX Group Moderation Bot
**Author:** Sascha Daemgen / IT and More Systems
**Date:** 2026-03-31 (updated 2026-04-04)
**Status:** Season 1 Complete, Season 2 Planned

---

## 1. Overview

GoBot is a moderation and verification bot for SimpleX Chat groups,
available in two deployment models:

**Hardware Model:** GoBot runs as a native FreeRTOS task on ESP32-S3
hardware, leveraging SimpleGo's complete C implementation of the SMP
protocol (47 files, 21,863 LOC). The bot reads from rx_ring_buffer,
writes to tx_ring_buffer, and shares the existing network and crypto
stack. Secured with eFuse-burned Secure Boot, Flash Encryption, and
disabled JTAG. No server, no SSH, no OS attack surface.

**Server Model:** GoBot runs as a Node.js process using the official
simplex-chat npm package (native FFI to Haskell core). Self-hosted
on Linux alongside the SMP server or separately. Deployed and tested
in Season 1.

Both models communicate through the standard SMP protocol - fully E2E
encrypted, indistinguishable from a regular group member. GoBot's
primary role is enforcing GoUNITY verified identities in SimpleX groups.

---

## 2. System architecture

### 2.1 Hardware Model (ESP32-S3 Native)

GoBot integrates directly into the SimpleGo firmware as an additional
FreeRTOS task. No CLI, no WebSocket, no external process.

```
+---------------------------------------------------------------+
|                     ESP32-S3 (SimpleGo Firmware)              |
|                                                               |
|  Core 0:                                                      |
|  +------------------+  +------------------+                   |
|  | network_task     |  | wifi_manager     |                   |
|  | 16 KB SRAM       |  | 4 KB PSRAM       |                   |
|  | TLS 1.3 / SMP    |  | Multi-network    |                   |
|  +--------+---------+  +------------------+                   |
|           |                                                   |
|  Core 1:  |                                                   |
|  +--------v---------+  +------------------+  +--------------+ |
|  | smp_app_task     |  | gobot_task       |  | GoUNITY      | |
|  | 16 KB SRAM       |  | 8 KB SRAM        |  | Verifier     | |
|  |                  |  |                  |  |              | |
|  | Double Ratchet   |  | Command parsing  |  | Ed25519 sig  | |
|  | NaCl crypto      |  | Moderation logic |  | CRL check    | |
|  | Contact mgmt     |  | Ban enforcement  |  | Level check  | |
|  | NVS persistence  |  | Rate limiting    |  |              | |
|  +--------+---------+  +--------+---------+  +------+-------+ |
|           |                      |                   |         |
|  +--------v-----------------------------------------v-------+ |
|  |              Inter-Task Communication                    | |
|  |  rx_ring_buffer: Network -> App -> GoBot (read msgs)     | |
|  |  tx_ring_buffer: GoBot -> App -> Network (send replies)  | |
|  |  NVS Flash: Ban lists, warnings, verified users          | |
|  +----------------------------------------------------------+ |
|                                                               |
|  Security Layer:                                              |
|  Secure Boot v2 (RSA-3072) | Flash Encrypt (AES-256-XTS)     |
|  JTAG disabled (eFuse)     | ATECC608B Secure Element        |
+---------------------------------------------------------------+
```

**Key advantage:** No display task needed. SimpleGo's lvgl_task (8 KB
stack + 64 KB LVGL pool + 25.6 KB draw buffers) is not loaded, freeing
approximately 100 KB of RAM for bot logic, ban databases, and message
processing buffers.

**Memory budget (without display):**

| Resource | Available | GoBot usage | Remaining |
|----------|-----------|-------------|-----------|
| Internal SRAM | ~330 KB free | ~50 KB (task + buffers) | ~280 KB |
| PSRAM | 8 MB | ~200 KB (ban DB + cache) | ~7.8 MB |
| NVS Flash | 128 KB | ~40 KB (persistent state) | ~88 KB |

### 2.2 Server Model (Node.js SDK)

GoBot wraps the simplex-chat npm package which embeds the Haskell
core via native FFI. No CLI process, no WebSocket.

```
+---------------------------------------------------------------+
|                        GoBot Server                           |
|                                                               |
|  +------------------+  +------------------+  +--------------+ |
|  | Bot Engine       |  | Moderation       |  | GoUNITY      | |
|  | (TypeScript)     |  | Engine           |  | Verifier     | |
|  |                  |  |                  |  |              | |
|  | Message parsing  |  | Ban/mute/warn    |  | Ed25519 sig  | |
|  | Command routing  |  | Auto-moderation  |  | CRL check    | |
|  | Event handling   |  | Report handling  |  | Level check  | |
|  +--------+---------+  +--------+---------+  +------+-------+ |
|           |                      |                   |         |
|  +--------v-----------------------------------------v-------+ |
|  |                  State Manager                           | |
|  |  Per-group: members, bans, mutes, warnings, config       | |
|  |  Storage: SQLite                                          | |
|  +----------------------------+-----------------------------+ |
|                               |                               |
+-------------------------------+-------------------------------+
                                |
                    +-----------v-----------+
                    |  simplex-chat SDK     |
                    |  (Haskell FFI)        |
                    +-----------+-----------+
                                |
                    +-----------v-----------+
                    |  SMP Network          |
                    |  (E2E encrypted)      |
                    +-----------------------+
```

---

## 3. GoUNITY certificate verification

### 3.1 Season 2 approach: One-time verification codes

For Season 2, GoUNITY uses a simpler but secure approach:

```
1. User visits id.simplego.dev/register
2. User creates account (email verification)
3. User requests verification code
4. GoUNITY generates single-use code (UUID + HMAC)
5. User sends code to GoBot via DM: /verify <code>
6. GoBot validates code via HTTPS call to GoUNITY API
7. GoUNITY marks code as consumed (single-use)
8. GoBot stores verified username locally
```

Advantages over full certificates:
- No certificate sharing possible (code is single-use)
- No replay attacks (code consumed on first use)
- Simpler UX (no key management for users)
- Trade-off: GoUNITY must be online during verification

### 3.2 Future: Full Ed25519 certificates with challenge-response

```
1. User receives Ed25519 keypair + signed certificate
2. GoBot sends random nonce via DM
3. User signs nonce with private key (requires client tool)
4. GoBot verifies: certificate signature + nonce signature
5. Proves: user holds the private key, not just the cert text
```

This eliminates the online requirement but needs user-side tooling.

### 3.3 Certificate caching

Once verified, GoBot stores the mapping locally:

```sql
-- verified_users table (SQLite on Server, NVS on Hardware)
  group_id    TEXT
  simplex_id  TEXT     -- SimpleX internal member ID
  username    TEXT     -- GoUNITY verified name
  level       INTEGER  -- verification level
  verified_at TIMESTAMP
```

GoBot does NOT store certificates, queue addresses, or message content.

---

## 4. Moderation engine

### 4.1 Permission system (Season 2 priority)

Every mod command checks the sender's SimpleX role before execution:

```
incoming command -> extract sender GroupMember -> check memberRole
  owner     -> all commands allowed
  admin     -> all commands except /mode
  moderator -> kick, warn, mute, reports
  member    -> /verify, /report, /rules, /mystatus only
  author    -> same as member
  observer  -> /verify only
```

The role check uses the GroupMember object that SimpleX includes
with every incoming group message (groupRcv direction). This is
built into the protocol - no additional API call needed.

### 4.2 Auto-moderation rules

```
Configurable per group:

  Spam:        Message frequency > threshold -> auto-mute
  Flood:       Messages/hour > limit -> auto-restrict
  Cooldown:    New members read-only for N minutes
  File block:  No files/links for first 24 hours
  Escalation:  3 auto-mutes -> auto-ban (if GoUNITY linked)
```

### 4.3 State persistence

**Server Model:** SQLite database with tables for members, bans,
mutes, warnings, reports, and config.

**Hardware Model:** NVS Flash for critical state (ban list, verified
users). NVS is encrypted when Flash Encryption is enabled. Limited
to ~88 KB usable space, sufficient for ~500 ban entries + ~200
verified user mappings.

---

## 5. The bot security paradox

### 5.1 The fundamental problem

Any bot with admin rights in an E2E encrypted group receives all
messages in cleartext. This is inherent to the E2E model. The bot
IS an endpoint. Transport encryption protects the pipe, but the
bot is not the pipe.

On a VPS: SSH compromise = full group surveillance. The attacker
can read process memory, modify code, inject logging, exfiltrate
via network. DB encryption is irrelevant because the bot holds the
key in memory while running.

### 5.2 How the Hardware Model changes this

On ESP32-S3 with eFuses burned:
- No SSH, no shell, no remote access of any kind
- Firmware verified at every boot (Secure Boot v2)
- Flash encrypted (AES-256-XTS, key in eFuse, hardware-only access)
- JTAG permanently disabled (eFuse burned)
- Code cannot be modified without replacing the entire chip
- No logging code in firmware (verifiable via reproducible build)
- Messages exist in RAM only during processing, then overwritten

The attacker model shifts from "anyone with a password" to "someone
with physical access AND lab equipment for side-channel analysis."
This is a fundamentally different threat level.

### 5.3 Known hardware attack vectors

ESP32-S3 is NOT a certified secure element. Known attacks on ESP32 family:
- Side-channel power analysis: broke ESP32-V3/C3/C6 AES with ~60K
  measurements (~$100 equipment). ESP32-S3 improved but not confirmed immune.
- Voltage glitching on eFuse shadow register reads during boot
- Body-biased fault injection (Espressif confirms no hardware fix)
- Physical die decapping with microprobing (requires depackaging)

Mitigations in GoBot hardware design:
- External secure element (ATECC608B) for high-value identity keys
- Conductive mesh overlay connected to GPIO interrupt (tamper detect)
- Epoxy potting over SoC and flash chip
- Triple-vendor secure elements planned for production PCB
  (ATECC608B + OPTIGA Trust M + SE050)

### 5.4 Security comparison

| Threat | VPS Bot | Hardware Bot |
|:-------|:--------|:-------------|
| Remote SSH attack | VULNERABLE | IMMUNE (no SSH) |
| OS/kernel exploit | VULNERABLE | IMMUNE (no OS) |
| Code injection | VULNERABLE | IMMUNE (Secure Boot) |
| Process memory dump | VULNERABLE | IMMUNE (no debug) |
| Flash readout | N/A | PROTECTED (AES-256-XTS) |
| Network exfiltration | VULNERABLE | PROTECTED (no general networking) |
| Physical side-channel | N/A | PARTIALLY VULNERABLE |
| Physical decapping | N/A | VULNERABLE (lab equipment) |

---

## 6. Open questions resolved from Season 1

1. **simplex-chat CLI API stability:** Not relevant for Server Model
   (using native FFI SDK). Not relevant for Hardware Model (native C).

2. **Rate limits:** SMP servers do not impose bot-specific rate limits.
   The bot is a regular client. Practical group limit ~100-200 members
   due to O(n^2) connections.

3. **Group admin API:** Confirmed via SDK types: apiRemoveMembers,
   apiBlockMembersForAll, apiMembersRole, apiAcceptMember all exist.
   apiRemoveMembers and apiBlockMembersForAll not yet tested.

4. **Multi-device:** simplex-chat persists state in SQLite. Bot survives
   restarts. One instance per database (no multi-device).

5. **Certificate transport:** Season 2 will use one-time codes via DM.
   Full certificates with challenge-response deferred to Season 3+.

---

*GoBot Technical Concept v2 - April 2026*
*IT and More Systems, Recklinghausen, Germany*
