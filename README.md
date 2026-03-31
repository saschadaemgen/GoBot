<p align="center">
  <strong>GoBot</strong>
</p>

<p align="center">
  <strong>Automated moderation and verification bot for SimpleX groups.</strong><br>
  Works with the standard SimpleX app. No plugins. No modifications. Just add GoBot to your group.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg" alt="License"></a>
  <a href="#status"><img src="https://img.shields.io/badge/status-concept-yellow.svg" alt="Status"></a>
  <a href="https://github.com/saschadaemgen/SimpleGo"><img src="https://img.shields.io/badge/ecosystem-SimpleGo-green.svg" alt="SimpleGo"></a>
</p>

---

GoBot is a moderation and automation bot for SimpleX Chat groups. It runs as a headless SimpleX client on a server, joins groups as a member with admin privileges, and provides verification, moderation, and automation services that SimpleX groups currently lack.

GoBot is the enforcement arm of [GoUNITY](https://github.com/saschadaemgen/GoUNITY) - the verified identity system for SimpleX. While GoUNITY issues verified username certificates, GoBot enforces them in groups: checking certificates, maintaining ban lists, and executing moderation actions. Together, they solve SimpleX's biggest unsolved problem - effective group moderation without breaking privacy.

**The key insight:** GoBot works with the standard, unmodified SimpleX app. Users don't need to install anything special. They just interact with GoBot through normal chat messages. This means GoUNITY verification works for every SimpleX user on every platform - phone, desktop, CLI - today, without waiting for upstream changes.

---

## The problem GoBot solves

SimpleX groups today have no effective moderation tools:

- **Ban evasion:** Banned users rejoin in seconds with a new profile
- **No automation:** Admins must manually review every new member and every report
- **No verification:** No way to distinguish legitimate users from spam accounts
- **No rate limiting:** Spammers can flood groups faster than admins can react
- **No cross-group coordination:** Each group's ban list starts from zero

GoBot turns SimpleX groups from unmoderable anonymous spaces into well-run communities - while keeping the privacy that makes SimpleX valuable.

---

## How GoBot works

### The doorman

When a new user joins a GoBot-moderated group, GoBot acts as a doorman:

```
User joins group
  |
  v
GoBot: "Welcome! This group requires GoUNITY verification.
        Please send your certificate to verify your identity.
        Get one at https://id.simplego.dev/register"
  |
  v
User sends certificate (copy-paste from GoUNITY website)
  |
  v
GoBot verifies:
  1. Ed25519 signature valid?          -> YES
  2. Certificate expired?              -> NO
  3. Username on ban list?             -> NO
  4. Minimum verification level met?   -> YES
  |
  v
GoBot: "Verified! Welcome, MeinPrinz."
GoBot grants full messaging permissions.
```

For groups in "mixed mode", unverified users can still participate but with restrictions (message limits, no files, no links).

### Moderation commands

Group admins interact with GoBot through simple chat commands:

```
Admin commands (in group or DM to GoBot):

/ban MeinPrinz harassment         Ban user with reason
/mute MeinPrinz 24h               Mute user for 24 hours
/restrict MeinPrinz 5/h           Limit to 5 messages per hour
/warn MeinPrinz                   Issue a warning (tracked)
/unban MeinPrinz                  Remove ban
/unmute MeinPrinz                 Remove mute
/status MeinPrinz                 Show user's moderation history
/banlist                          Show all active bans
/reports                          Show pending user reports
/mode verified                    Set group to verified-only
/mode mixed                       Set group to mixed mode
/mode open                        Set group to open (no verification)
/help                             Show all commands
```

### User commands

Regular users can also interact with GoBot:

```
User commands (DM to GoBot):

/verify <certificate>             Submit GoUNITY certificate
/report MeinPrinz spam            Report a user to admins
/mystatus                         Check your own verification status
/rules                            Show group rules
```

### Automated moderation

GoBot can act autonomously based on configurable rules:

```
Auto-moderation rules:

  Spam detection:
    [x] Auto-mute after 10 messages in 60 seconds
    [x] Auto-delete messages with known spam patterns
    [ ] Auto-ban after 3 auto-mutes
    
  New member restrictions:
    [x] Read-only for first 5 minutes
    [x] No files for first 24 hours
    [x] No links for first 24 hours
    
  Flood protection:
    [x] Max 30 messages per hour per user
    [x] Max 5 images per hour per user
    [ ] Slow mode: 1 message per 30 seconds
```

---

## Architecture

### How GoBot connects to SimpleX

GoBot runs as a headless SimpleX client using the `simplex-chat` CLI in terminal mode. It communicates with the SimpleX network just like any other client - through SMP queues with full E2E encryption.

```
+------------------+     +------------------+     +------------------+
|    GoBot         |     |   SMP Server     |     |   Group Members  |
|    (server)      |     |                  |     |   (SimpleX App)  |
|                  |     |                  |     |                  |
|  simplex-chat    |<--->|  Message queues  |<--->|  Standard app    |
|  (CLI mode)      |     |  E2E encrypted   |     |  No mods needed  |
|                  |     |                  |     |                  |
|  GoBot logic     |     |                  |     |                  |
|  (Go/Python)     |     |                  |     |                  |
+------------------+     +------------------+     +------------------+
         |
         | HTTPS (periodic)
         v
+------------------+
|  GoUNITY SIS     |
|                  |
|  Certificate     |
|  Revocation List |
|  (CRL)           |
+------------------+
```

### Technology stack

| Component | Technology | Reason |
|:----------|:-----------|:-------|
| Bot runtime | Go | Matches ecosystem, concurrent, single binary |
| SimpleX interface | simplex-chat CLI | Battle-tested, full protocol support |
| CLI communication | JSON over stdin/stdout | SimpleX CLI's native API mode |
| Certificate verification | Ed25519 (Go crypto) | Local verification, no network needed |
| Configuration | YAML | Human-readable, easy to edit |
| Database | SQLite | Lightweight, zero-config, per-group storage |
| CRL updates | HTTPS fetch | Periodic (daily) from GoUNITY SIS |
| Hosting | Any Linux server | Minimal resources (1 CPU, 512MB RAM) |

### SimpleX CLI API mode

The simplex-chat CLI supports a JSON API mode where it reads commands from stdin and writes events to stdout. GoBot wraps this interface:

```
GoBot                           simplex-chat CLI
  |                                    |
  |-- {"cmd": "sendMessage",    ----->|
  |    "group": "MyGroup",            |
  |    "text": "Welcome!"}            |
  |                                    |
  |<-- {"event": "newMessage",  ------|
  |    "group": "MyGroup",            |
  |    "from": "User123",             |
  |    "text": "/verify abc..."}      |
  |                                    |
```

---

## GoUNITY integration

GoBot is the bridge between GoUNITY certificates and SimpleX groups.

### Certificate flow

```
1. User registers "MeinPrinz" at id.simplego.dev
2. User receives Ed25519 signed certificate
3. User joins SimpleX group (standard app)
4. GoBot asks for certificate
5. User sends certificate as text message to GoBot
6. GoBot verifies signature locally (no GoUNITY contact)
7. GoBot stores: username -> group member mapping (local only)
8. GoBot grants permissions based on verification level

GoUNITY server is NEVER contacted during verification.
GoUNITY never learns which groups the user joins.
```

### Ban enforcement

```
Admin: /ban MeinPrinz harassment

GoBot:
  1. Adds "MeinPrinz" to local ban list
  2. Removes user from group
  3. If "MeinPrinz" tries to rejoin:
     a. Submits certificate -> GoBot checks ban list -> REJECTED
     b. No certificate -> Group is verified-only -> REJECTED
     c. New certificate "DifferentName" -> Allowed (new identity, new chance)
        But costs 5-30 EUR + new verification
```

### CRL synchronization

GoBot periodically fetches the Certificate Revocation List from GoUNITY:

```
Daily (configurable):
  1. GoBot fetches https://id.simplego.dev/v1/crl
  2. CRL contains: list of revoked usernames + GoUNITY signature
  3. GoBot verifies CRL signature
  4. GoBot cross-references with group members
  5. Revoked members get notified and optionally removed
```

---

## Multi-group support

A single GoBot instance can moderate multiple groups simultaneously:

```yaml
# gobot.yaml
groups:
  - name: "SimpleGo Community"
    mode: verified
    min_level: basic
    auto_moderation:
      spam_detection: true
      flood_protection: true
      new_member_cooldown: 5m
    admins:
      - "Sascha"
      - "Moderator1"
      
  - name: "GoShop Sellers"
    mode: verified
    min_level: business
    auto_moderation:
      spam_detection: true
    admins:
      - "Sascha"
      
  - name: "Casual Chat"
    mode: mixed
    restrictions_unverified:
      messages_per_hour: 10
      send_files: false
    admins:
      - "Sascha"
      - "Moderator2"
```

---

## Self-hosting

GoBot is designed to be self-hosted. Any group admin can run their own instance:

```bash
# Quick start
git clone https://github.com/saschadaemgen/GoBot.git
cd GoBot
cp gobot.example.yaml gobot.yaml    # Edit configuration
./gobot --config gobot.yaml          # Start the bot

# Or with Docker
docker run -d \
  -v ./gobot.yaml:/app/gobot.yaml \
  -v ./data:/app/data \
  ghcr.io/saschadaemgen/gobot:latest
```

**Requirements:**
- Linux server (Ubuntu 22.04+ / Debian 12+)
- simplex-chat CLI installed
- 1 CPU core, 512 MB RAM, 1 GB disk
- Outbound internet for SMP connections

---

## Roadmap

| Phase | Focus | Status |
|:------|:------|:-------|
| 0 | Concept + Architecture (this document) | DONE |
| 1 | Core bot: SimpleX CLI integration, message handling | Planned |
| 2 | GoUNITY integration: certificate verification, ban enforcement | Planned |
| 3 | Moderation commands: ban/mute/restrict/warn/report | Planned |
| 4 | Auto-moderation: spam detection, flood protection, cooldowns | Planned |
| 5 | Multi-group: single instance managing multiple groups | Planned |
| 6 | Admin dashboard: web UI for configuration and monitoring | Future |
| 7 | GoShop integration: order verification, seller trust badges | Future |

---

## Comparison with existing bots

| Feature | Telegram bots | Discord bots | SimpleX (now) | GoBot |
|:--------|:-------------|:-------------|:--------------|:------|
| Group moderation | Mature ecosystem | Mature ecosystem | None | Planned |
| Ban enforcement | Effective (phone-linked) | Effective (account-linked) | Broken (profile switch) | Effective (certificate-linked) |
| Identity verification | Phone number | Account + roles | None | GoUNITY certificates |
| Privacy preserved | No (phone exposed) | No (account tracked) | Yes (too much) | Yes (cryptographic separation) |
| Self-hostable | Via Bot API | Via Bot API | N/A | Full self-hosting |
| E2E encrypted | No | No | Yes | Yes |

GoBot is the first moderation bot that provides **both** effective moderation **and** privacy preservation. Every other solution sacrifices one for the other.

---

## SimpleGo ecosystem

GoBot is the automation layer of the SimpleGo ecosystem.

| Project | What it does | Repository |
|:--------|:-------------|:-----------|
| **[SimpleGo](https://github.com/saschadaemgen/SimpleGo)** | Dedicated hardware messenger on ESP32-S3 | [SimpleGo](https://github.com/saschadaemgen/SimpleGo) |
| **[GoRelay](https://github.com/saschadaemgen/GoRelay)** | Encrypted relay server (SMP + GRP) | [GoRelay](https://github.com/saschadaemgen/GoRelay) |
| **[GoChat](https://github.com/saschadaemgen/GoChat)** | Browser-native encrypted chat plugin | [GoChat](https://github.com/saschadaemgen/GoChat) |
| **[GoShop](https://github.com/saschadaemgen/GoShop)** | End-to-end encrypted e-commerce | [GoShop](https://github.com/saschadaemgen/GoShop) |
| **[GoBot](https://github.com/saschadaemgen/GoBot)** | Moderation + automation bot (this project) | [GoBot](https://github.com/saschadaemgen/GoBot) |
| **[GoUNITY](https://github.com/saschadaemgen/GoUNITY)** | Verified identity + moderation | [GoUNITY](https://github.com/saschadaemgen/GoUNITY) |

---

## License

AGPL-3.0

---

<p align="center">
  <i>GoBot is part of the <a href="https://github.com/saschadaemgen/SimpleGo">SimpleGo ecosystem</a> by IT and More Systems, Recklinghausen, Germany.</i>
</p>

<p align="center">
  <strong>GoBot - effective moderation for encrypted groups. No compromises on privacy.</strong>
</p>
