# GoBot - Season 1 Protocol

**Season:** 1
**Period:** March 31 - April 4, 2026
**Status:** Complete
**Version at close:** v0.0.1-alpha (TypeScript prototype, research only)

---

## Summary

Season 1 established the foundation for GoBot - a hardware-secured
moderation bot for SimpleX Chat groups. We researched the SimpleX
bot ecosystem, built a TypeScript prototype to learn the API,
discovered the fundamental bot security paradox, and designed a
split-crypto architecture (GoBot + GoKey + GoUNITY) that is novel
enough to be publishable as an academic paper.

The season ended with a major architecture pivot: from "bot runs
natively on ESP32" to "Go proxy on VPS + ESP32 crypto engine at
home" after the SimpleGo team identified that holding multiple
TLS connections on an ESP32 is impractical for group moderation.

---

## Architecture evolution during Season 1

### Phase 1: Server bot (TypeScript)
Built a working bot with simplex-chat npm SDK to learn the API.
All commands working, deployed on VPS. But: server compromise =
full group surveillance.

### Phase 2: Native ESP32 bot
Idea: run GoBot as FreeRTOS task in SimpleGo firmware. ESP32 has
the complete SMP crypto stack. eFuse seals everything. But: each
SMP connection needs ~40 KB SRAM for TLS. Groups with members on
different servers need multiple connections. ESP32 has 512 KB total.
Maximum ~8 connections. Not enough for real groups.

### Phase 3: Split architecture (final)
SimpleGo team insight: separate network (VPS) from crypto (ESP32).
VPS holds all TLS connections (Go handles thousands easily). ESP32
at home holds all keys and does all decrypt/encrypt via one WSS
connection to the VPS. Server never sees plaintext. Best of both.

This is the architecture going forward.

---

## What was accomplished

### Research

- Complete SimpleX bot ecosystem analysis (14+ known projects)
- Evaluated three SDK approaches (CLI WebSocket, TS client, native FFI)
- Documented complete SimpleX Bot API from @simplex-chat/types v0.3.0
- Discovered the bot security paradox (bot in E2E group = backdoor)
- Analyzed SimpleX Directory Bot (stores all messages, no audit coverage)
- Compared group discovery across Signal, Matrix, Session, Telegram
- Researched hardware security: ESP32-S3 eFuse, Secure Boot, side-channel attacks
- Researched TEE approaches: Signal SGX, Apple PCC, AWS Nitro Enclaves
- Researched privacy-preserving moderation: message franking, homomorphic filtering
- Identified step-ca (smallstep/certificates) as base for GoUNITY
- Independent security review of split-crypto architecture

### Security findings

**Response Oracle:** Binary IGNORE/CMD response pattern leaks which
messages are commands. Fix: constant-size, constant-time responses
with dummy blocks. Built into wire protocol design.

**Command Replay:** Ed25519 signatures are deterministic. Without
freshness, commands can be replayed. Fix: sequence number + timestamp
+ group ID + block hash in every signed command.

**ChaCha20 over AES:** ESP32-S3 hardware AES vulnerable to side-channel
power analysis. ChaCha20-Poly1305 in software is 3x faster and
immune to power analysis.

**ESP32 physical attacks:** Side-channel confirmed on ESP32-V3/C3/C6
(~60K power traces, ~$100 equipment). ESP32-S3 unconfirmed but
likely similar. Mitigated by ATECC608B secure element.

### Implementation (TypeScript prototype)

- GoBot v0.0.1-alpha built with simplex-chat npm SDK
- Commands: !help, !time, !date, !status, !ping, !members,
  !kick, !warn, !warnings, !clearwarn
- Auto-accept contacts and group invitations
- Per-user rate limiting (sliding window)
- Avatar via apiUpdateProfile() (max ~12.5 KB, 192x192 JPEG)
- Deployed as systemd service on VPS (smp.simplego.dev)
- All GroupMember types verified against running bot

### Verified SimpleX API methods

| Method | Tested |
|:-------|:-------|
| apiSendTextReply | YES |
| apiListMembers | YES |
| apiJoinGroup | YES |
| apiUpdateProfile | YES |
| apiRemoveMembers | NO |
| apiBlockMembersForAll | NO |
| apiMembersRole | NO |
| apiAcceptMember | NO |

### Key technical facts discovered

- GroupMember.memberRole: owner/admin/moderator/member/author/observer
- memberContactId: only stable cross-group identifier
- localDisplayName: NOT stable, user can change anytime
- Groups: O(n) sends, O(n^2) connections, limit ~100-200 members
- Avatar: bot.run() ignores image field, must use apiUpdateProfile()
- SimpleX has no persistent user identity (why GoUNITY exists)

### Documentation created

- SYSTEM-ARCHITECTURE.md (complete system with wire protocol)
- ARCHITECTURE_AND_SECURITY.md (GoBot Go service specific)
- CONCEPT.md (technical concept v3, final architecture)
- SIMPLEX-BOT-API-REFERENCE.md (verified types and methods)
- GoUNITY README.md and ARCHITECTURE_AND_SECURITY.md
- GoBot README.md (final version with security findings)

### Repositories established

| Repo | Purpose |
|:-----|:--------|
| github.com/saschadaemgen/GoBot | Main project, Go service, docs |
| github.com/saschadaemgen/GoUNITY | Certificate authority (step-ca fork) |
| SimpleGo template (GoKey) | ESP32 crypto engine (planned) |

---

## Lessons learned

1. **Every bot is a backdoor.** The moment you add a bot to an E2E
   group, your encryption guarantee drops to the security of the
   bot's execution environment. This is not fixable - only mitigatable.

2. **The split-crypto architecture works.** Separating network from
   crypto is a proven pattern (Cloudflare, Qubes, FIDO2, banking).
   First application to messenger bots.

3. **ESP32 cannot hold many TLS connections.** ~40 KB per connection,
   512 KB total. Groups need multiple server connections. The proxy
   architecture solves this elegantly.

4. **step-ca saves months of work.** Production-grade CA with Ed25519,
   CRL, HSM, OIDC, API, templates. Fork and customize instead of
   building from scratch.

5. **The response oracle is subtle.** Size and timing differences
   between IGNORE and CMD responses leak information. Must be
   designed out from the start, not patched later.

6. **ChaCha20 > AES on ESP32.** 3x faster, constant-time, no
   vulnerable hardware accelerator. Counter-intuitive but proven.

7. **Season-based development works.** Focused increments with
   protocol and handoff documents prevent scope creep and
   knowledge loss.

---

## Known issues carried into Season 2

- TypeScript prototype has no permission checking (any user can !kick)
- Warnings stored in RAM only (lost on restart)
- No persistent ban list
- German filenames (hilfe.ts, zeit.ts, datum.ts)
- No GoUNITY integration
- Several API methods untested (apiRemoveMembers, etc.)
- TypeScript prototype is research only - production will be Go

---

## Infrastructure at close of Season 1

| Component | Location |
|:----------|:---------|
| TypeScript prototype | /opt/gobot/ on smp.simplego.dev |
| Bot service | gobot.service (systemd) |
| Bot databases | /opt/gobot/data/nwbot_*.db |
| GoBot repo | github.com/saschadaemgen/GoBot |
| GoUNITY repo | github.com/saschadaemgen/GoUNITY |
| Bot address | smp15.simplex.im/a#m36LEAL7T... |

---

*Season 1 closed April 4, 2026.*
*Season 2: GoBot Go service, GoKey Wire Protocol.*
