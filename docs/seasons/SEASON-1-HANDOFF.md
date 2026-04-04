# GoBot - Season 2 Handoff Document

**From:** Season 1 (closed April 4, 2026)
**To:** Season 2
**Project:** GoBot System (GoBot + GoKey + GoUNITY)
**License:** AGPL-3.0 (GoBot, GoKey) / Apache-2.0 (GoUNITY)

---

## CRITICAL RULES

1. **NEVER change version numbers** without explicit permission
2. **English** for all code, output, and documentation
3. **No em dashes** - use hyphens or rewrite
4. **Conventional Commits:** `type(scope): description`
5. **Season-based development** with protocol and handoff docs
6. **Season/Plan/Handoff documents are INTERNAL** - never committed to repos

---

## THE SYSTEM IN 30 SECONDS

GoBot is a moderation bot for SimpleX Chat where the server
never sees a single message.

**GoBot** (Go service on VPS) holds SMP connections, forwards
encrypted 16 KB blocks, executes signed commands. Dumb proxy.
Cannot decrypt anything.

**GoKey** (ESP32-S3 at home) holds all keys (eFuse), decrypts
messages (3-4 ms), checks for commands, signs results, encrypts
replies. Secure core. Stecker ziehen = Bot tot.

**GoUNITY** (step-ca fork) issues Ed25519 certificates for user
verification with challenge-response. Bans linked to certificates.

GoBot without GoKey works standalone (lower security, ~30-40%).
With GoKey: ~85-90% of SimpleX security preserved.

---

## REPOSITORIES

| Repo | URL | Language | Contains |
|:-----|:----|:---------|:---------|
| GoBot | github.com/saschadaemgen/GoBot | Go (Season 2+) / TS (Season 1 prototype) | Proxy service, docs, season protocols |
| GoUNITY | github.com/saschadaemgen/GoUNITY | Go | step-ca fork, CA for identity verification |
| GoKey | Template in SimpleGo repo | C (ESP-IDF) | ESP32 crypto engine (Season 3) |

---

## WHAT EXISTS RIGHT NOW

### GoBot Repo

```
C:\Projects\GoBot\
  README.md                              # Final (split-crypto architecture)
  LICENSE                                 # AGPL-3.0
  docs/
    SYSTEM-ARCHITECTURE.md               # Full system (GoBot+GoKey+GoUNITY)
    ARCHITECTURE_AND_SECURITY.md          # GoBot Go service specific
    CONCEPT.md                            # Technical concept v3
    SIMPLEX-BOT-API-REFERENCE.md          # Verified SimpleX types/methods
    SEASON-1-PROTOCOL.md                  # Season 1 complete protocol
  gobot/                                  # TypeScript prototype (Season 1 only)
    src/                                  # NOT the production code
    package.json                          # Will be replaced by Go in Season 2
```

### GoUNITY Repo

```
C:\Projects\GoUNITY\
  README.md                              # GoUNITY overview + setup
  docs/
    ARCHITECTURE_AND_SECURITY.md          # CA security, certificate lifecycle
  (+ all step-ca source from fork)
```

### Server (TypeScript prototype - research only)

```
Host: smp.simplego.dev (SSH as root)
Path: /opt/gobot/
Service: gobot.service
Bot address: smp15.simplex.im/a#m36LEAL7T...
WARNING: nwbot_*.db deletion = permanent identity loss
```

---

## ARCHITECTURE

```
[VPS: GoBot Go Service]              [Home: GoKey ESP32-S3]
Holds SMP/TLS connections             All private keys (eFuse)
Receives encrypted blocks             All ratchet state (NVS)
Cannot decrypt                         Decrypts (3-4 ms)
  |                                         |
  |-- encrypted block -----WSS/mTLS------->|
  |                                    Parse: command?
  |                                    YES: sign result
  |                                    NO: dummy block
  |                                    (constant-size, constant-time)
  |<-- signed response ----WSS/mTLS-------|
  |
  Verify signature + sequence
  Execute command via SMP

[GoUNITY: step-ca Fork]
Ed25519 certificates
CRL distribution (HTTPS)
Challenge-response verification
HSM-backed CA key (YubiKey)
```

---

## CRITICAL SECURITY FIXES (built into design)

### 1. Response Oracle

Every response from GoKey is constant-size, constant-time.
Non-commands generate dummy 16 KB blocks. Random delay 100-500ms.
VPS cannot distinguish commands from non-commands.

### 2. Command Replay

Signed command format:
```
SIGN(seq_num || timestamp || group_id || block_hash || command)
```
Monotonic sequence, 30-second timestamp window, context binding.
Each signature unique and non-replayable.

### 3. ChaCha20 over AES

ESP32-S3 hardware AES vulnerable to side-channel power analysis.
ChaCha20-Poly1305 in software: 3x faster, constant-time, immune.

---

## SIMPLEX API (verified in Season 1)

| Method | Tested | Purpose |
|:-------|:-------|:--------|
| apiSendTextReply | YES | Bot responses |
| apiListMembers | YES | Permission checks |
| apiJoinGroup | YES | Auto-join groups |
| apiUpdateProfile | YES | Avatar, bot indicator |
| apiRemoveMembers | NO | Kick/ban |
| apiBlockMembersForAll | NO | Shadow blocking |
| apiMembersRole | NO | Mute (set observer) |
| apiAcceptMember | NO | Doorman approval |

### GroupMember key fields

- `memberRole`: owner/admin/moderator/member/author/observer
- `memberContactId`: stable cross-group ID (if bot contact)
- `localDisplayName`: NOT stable, user can change
- `groupRcv` includes full GroupMember of sender

---

## SEASON 2 FOCUS

### Sprint 0: GoKey Wire Protocol specification

Define the exact protocol between GoBot and GoKey before writing
any code. JSON format, constant-size padding, signed commands,
heartbeat, acknowledgments, error handling.

### Sprint 1: GoBot Go project setup

New Go project in GoBot repo (replacing TypeScript prototype).
Module init, CI, basic structure, SMP frame-level client.

### Sprint 2: GoBot SMP proxy

Connect to SMP servers, subscribe to queues, receive encrypted
blocks. No crypto - just frame handling and block forwarding.

### Sprint 3: GoBot standalone mode

For testing without GoKey: decrypt locally, parse commands,
execute. Permission system (memberRole checks). Persistent
warnings and bans (SQLite).

### Sprint 4: GoBot WSS server for GoKey

WSS endpoint with mTLS. Forward blocks to GoKey, receive signed
responses, verify signatures, execute commands. Block buffering.
Heartbeat monitoring.

### Sprint 5: Documentation + Season close

Season 2 Protocol, Season 3 Handoff (GoKey ESP32 firmware).

---

## OUT OF SCOPE FOR SEASON 2

- GoKey ESP32 firmware (Season 3)
- GoUNITY integration (Season 4)
- Auto-moderation (Season 5)
- Multi-group management (Season 5)
- Web dashboard (Season 5+)
- Messenger bridging (deferred indefinitely)

---

## CONTEXT

### SimpleX ecosystem to monitor

- Email integration planned (3-5 months, Evgeny mentioned in group)
- v6.4.3 bot command menus in app UI
- Super-peers/chat relays in development
- simplex-chat npm SDK still beta (v6.5.0-beta.4.4)

### Directory Bot findings (INTERNAL)

Stores all messages in listed groups as architectural side effect.
Privacy policy openly states this. Neither audit covered it.
No reproducible build. Kept internal for strategic advantage.

### Collaborator: Szenni

Python/CLI bot builder. IRC experience. Identified bot security
problem independently. Wants web interface. His code is separate.

---

## CHEAT SHEET

```bash
# Server (TypeScript prototype - still running)
systemctl start|stop|restart|status gobot
journalctl -u gobot -f

# GoBot repo
cd C:\Projects\GoBot
git add -A && git commit -m "type(scope): desc" && git push

# GoUNITY repo
cd C:\Projects\GoUNITY
git add -A && git commit -m "type(scope): desc" && git push

# Check SimpleX types
grep -A 20 "interface GroupMember {" /opt/gobot/node_modules/@simplex-chat/types/dist/types.d.ts
```

---

## COMPARABLE ARCHITECTURES

| System | Pattern | Validates our approach |
|:-------|:--------|:----------------------|
| Cloudflare Keyless SSL | Edge proxy + key server | Same split, global scale |
| Qubes Split GPG | Network VM + crypto VM | Same isolation principle |
| FIDO2/WebAuthn | Browser + hardware key | Same key-never-leaves-device |
| Hardware wallets | App + secure element | Same companion model |
| Apple PCC | OHTTP relay + Secure Enclave | Same encrypted relay |
| Banking HSM | Terminal + HSM | Same pattern, decades proven |

Our architecture is the first application of this pattern to
E2E encrypted messenger bots. Security review confirms it is
novel and publishable.

---

*This document contains everything needed to start Season 2.*
