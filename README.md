<p align="center">
  <strong>GoBot</strong>
</p>

<p align="center">
  <strong>The world's first hardware-secured moderation bot for encrypted messaging.</strong><br>
  Runs natively on ESP32-S3 with eFuse-sealed firmware. No server. No SSH. No attack surface.<br>
  Also available as a self-hosted server deployment for standard infrastructure.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg" alt="License"></a>
  <a href="#status"><img src="https://img.shields.io/badge/status-alpha-orange.svg" alt="Status"></a>
  <a href="https://github.com/saschadaemgen/SimpleGo"><img src="https://img.shields.io/badge/ecosystem-SimpleGo-green.svg" alt="SimpleGo"></a>
</p>

---

GoBot is a moderation and verification bot for SimpleX Chat groups. It works with the standard, unmodified SimpleX app. Users don't need to install anything special - they just interact with GoBot through normal chat messages.

What makes GoBot unique: it is the first messenger bot designed to run on dedicated security hardware. Built on [SimpleGo's](https://github.com/saschadaemgen/SimpleGo) native C implementation of the SimpleX Messaging Protocol - the world's first non-Haskell SMP implementation (47 source files, 21,863 lines of C, verified against the official SimpleX Chat App) - GoBot runs as a FreeRTOS task on an ESP32-S3 microcontroller with Secure Boot, Flash Encryption, and permanently burned eFuses. Once deployed, not even the owner can modify the firmware or extract keys. The bot becomes a sealed, autonomous moderation appliance.

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

But there is a deeper problem that no other bot project addresses:

- **Every bot is a backdoor.** Any bot with admin rights in an E2E encrypted group receives all messages in cleartext. Commercial bots like Rose, Combot, and Shieldy centralize this risk across millions of groups on servers you don't own and can't audit. Even SimpleX's own Directory Bot stores all messages in listed groups as an architectural side effect of its chat client architecture. You can encrypt 4 layers or 40 layers - the bot is the endpoint, and it decrypts everything.

GoBot addresses both problems: effective moderation AND hardware-enforced security that eliminates the traditional bot attack surface.

---

## Two deployment models

### Model 1: Hardware Bot (SimpleGo Native)

GoBot runs as a FreeRTOS task directly on the ESP32-S3, leveraging SimpleGo's complete native C implementation of the SMP protocol. All four encryption layers (Double Ratchet X448, 2x NaCl cryptobox, TLS 1.3) run natively on the microcontroller. No external server, no CLI, no SSH, no operating system attack surface.

```
+---------------------------+     +------------------+     +------------------+
|  ESP32-S3 (SimpleGo HW)  |     |   SMP Server     |     |   Group Members  |
|                           |     |                  |     |   (SimpleX App)  |
|  [network_task] Core 0    |<--->|  Message queues  |<--->|  Standard app    |
|  TLS 1.3 + SMP transport  |     |  E2E encrypted   |     |  No mods needed  |
|                           |     |                  |     |                  |
|  [smp_app_task] Core 1    |     +------------------+     +------------------+
|  Double Ratchet, NaCl     |
|  Contact management       |
|                           |
|  [gobot_task] Core 1      |
|  Command parsing          |
|  Moderation engine        |
|  GoUNITY verification     |
|  Ban list (NVS Flash)     |
|                           |
|  SECURITY:                |
|  Secure Boot v2 (RSA-3072)|
|  Flash Encrypt (AES-256)  |
|  JTAG permanently disabled|
|  eFuses burned & locked   |
|  ATECC608B Secure Element |
+---------------------------+
```

**Security guarantees:**
- Firmware cannot be modified after eFuse burn (Secure Boot v2, RSA-PSS with RSA-3072)
- Flash contents cannot be read externally (AES-256-XTS encryption)
- No SSH, no shell, no debug interface (JTAG permanently disabled via eFuse)
- Identity keys stored in external secure element (ATECC608B, CC EAL6+ rated)
- No message logging possible (no logging code in sealed firmware, verifiable via reproducible build)
- Physical tamper detection via conductive mesh on GPIO interrupt
- Even the device owner cannot extract keys or modify behavior
- Without display task: ~100 KB additional RAM freed for bot logic

### Model 2: Server Bot (Node.js SDK)

GoBot runs as a Node.js process using the official `simplex-chat` npm package with the Haskell core embedded via native FFI. Self-hosted on your own server, alongside your SMP server or separately.

```
+------------------+     +------------------+     +------------------+
|    GoBot         |     |   SMP Server     |     |   Group Members  |
|    (VPS/Server)  |     |                  |     |   (SimpleX App)  |
|                  |     |                  |     |                  |
|  simplex-chat    |<--->|  Message queues  |<--->|  Standard app    |
|  Node.js SDK     |     |  E2E encrypted   |     |  No mods needed  |
|  (native FFI)    |     |                  |     |                  |
|                  |     |                  |     |                  |
|  GoBot logic     |     |                  |     |                  |
|  (TypeScript)    |     |                  |     |                  |
+------------------+     +------------------+     +------------------+
```

**Trade-offs:** Easier to deploy and update, but the server is a traditional attack surface. SSH compromise = full access to all group messages and moderation controls.

---

## The bot security problem - and why hardware matters

> "We use SimpleX - the most private messaging protocol on Earth. No user IDs, no metadata, double ratchet encryption, quantum-resistant, the whole nine yards. Then we invite a chatbot with admin rights that reads every message, lives on a Linux box with password 'changeme', and calls it 'group security'. That's like building a nuclear bunker and leaving the front door open because the pizza guy needs to get in."

Every group bot on every platform - Telegram, Discord, Matrix, SimpleX - is fundamentally a privacy compromise. The bot decrypts messages because it must. The question is only: who controls the decryption endpoint?

| Deployment | Attack surface | Who can read messages |
|:-----------|:--------------|:---------------------|
| Commercial hosted bot (Rose, Combot) | SSH, cloud provider, bot operator, law enforcement | Bot operator + anyone who compromises their servers |
| Self-hosted VPS bot (GoBot Server) | SSH, OS vulnerabilities, weak passwords | Server admin + anyone who compromises the server |
| **GoBot Hardware (ESP32-S3 + eFuse)** | **Physical access with lab equipment only** | **Nobody - firmware is sealed, no logging, no debug** |

The Hardware Model doesn't make the bot "more encrypted" - it makes the bot **tamper-proof**. The firmware is verified at boot, the flash is encrypted, debug interfaces are permanently disabled, and the code provably contains no message logging. This is a completely new category that has never existed before in the messenger bot space.

---

## Current status

GoBot v0.0.1-alpha (Server Model) is deployed and running. The Hardware Model is in design phase, leveraging SimpleGo's proven SMP implementation.

**Working right now (Server Model):**
- SimpleX Chat bot using the official `simplex-chat` Node.js SDK (native FFI, no CLI dependency)
- Auto-accept contact requests with configurable welcome message
- Auto-accept group invitations
- Group and direct message support
- Modular command system with register + dispatch pattern
- Per-user rate limiting (sliding window)
- Profile avatar support via `apiUpdateProfile`
- Runs as a systemd service with auto-restart

**Implemented commands:**

| Command | Description | Status |
|:--------|:-----------|:-------|
| !help | Show available commands | Working |
| !time | Current time (Europe/Berlin) | Working |
| !date | Current date | Working |
| !status | Bot uptime, memory, node version | Working |
| !ping | Check if bot is online | Working |
| !members | List group members | Working |
| !kick \<name\> | Remove a member from the group | Working |
| !warn \<name\> | Warn a member (3x = auto-kick) | Working |
| !warnings \<name\> | Check warnings for a member | Working |
| !clearwarn \<name\> | Clear warnings for a member | Working |

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

## Technology stack

### Hardware Model (ESP32-S3 Native)

| Component | Technology | Reason |
|:----------|:-----------|:-------|
| Hardware | ESP32-S3 (LilyGo T-Deck Plus or custom PCB) | Proven SimpleGo platform, dual-core 240 MHz, 8 MB PSRAM |
| Firmware | ESP-IDF 5.5.2 / FreeRTOS | Real-time OS, no Linux attack surface |
| SMP Protocol | SimpleGo native C (47 files, 21,863 LOC) | First non-Haskell SMP implementation worldwide |
| Encryption | X448 Double Ratchet + 2x NaCl + TLS 1.3 | 4 independent layers, verified against SimpleX App |
| Crypto libraries | mbedTLS + libsodium + wolfSSL | Hardware-accelerated AES, audited libraries |
| Key storage | ATECC608B + OPTIGA Trust M + SE050 (triple-vendor) | Three vendors: if one has a backdoor, the full key cannot be reconstructed |
| Tamper protection | Secure Boot v2 + Flash Encryption + eFuse | Firmware sealed permanently after burn |
| Bot state | NVS Flash (encrypted via eFuse-bound key) | Ban lists, warnings, verified users |

### Server Model (Node.js)

| Component | Technology | Reason |
|:----------|:-----------|:-------|
| Bot runtime | TypeScript / Node.js | Official SDK support, type safety, async/await |
| SimpleX interface | simplex-chat npm package | Native FFI to Haskell core, no CLI dependency |
| Database | SQLite (via SDK) | Managed by SimpleX core, encrypted with SQLCipher |
| Hosting | Any Linux server | Minimal resources (1 CPU, 512 MB RAM) |

---

## Self-hosting

### Server Model

```bash
git clone https://github.com/saschadaemgen/GoBot.git
cd GoBot/gobot
mkdir -p data
npm install
npm run build
npm start
```

### Hardware Model

```bash
# Requires SimpleGo development environment (ESP-IDF 5.5.2)
git clone https://github.com/saschadaemgen/GoBot.git
cd GoBot/gobot-hardware
idf.py set-target esp32s3
idf.py build
idf.py flash

# PRODUCTION: Burn security eFuses (IRREVERSIBLE)
# See docs/EFUSE-PROVISIONING.md
```

---

## Roadmap

| Phase | Focus | Status |
|:------|:------|:-------|
| 0 | Concept + Architecture | DONE |
| 1 | Core bot: SimpleX SDK integration, message handling, group commands, moderation basics (Server Model) | DONE |
| 2 | GoUNITY integration: certificate verification, ban enforcement | Planned |
| 3 | Advanced moderation: ban/mute/restrict/report, persistent warnings | Planned |
| 4 | Auto-moderation: spam detection, flood protection, cooldowns | Planned |
| 5 | Hardware Model: GoBot as FreeRTOS task in SimpleGo firmware | Planned |
| 6 | Multi-group: single instance managing multiple groups | Planned |
| 7 | Admin dashboard: web UI for configuration and monitoring | Future |
| 8 | GoShop integration: order verification, seller trust badges | Future |

---

## Comparison with existing bots

| Feature | Telegram bots | Discord bots | SimpleX (now) | GoBot (Server) | GoBot (Hardware) |
|:--------|:-------------|:-------------|:--------------|:--------------|:----------------|
| Group moderation | Mature | Mature | None | Working (alpha) | Planned |
| Ban enforcement | Phone-linked | Account-linked | Broken | Certificate-linked | Certificate-linked |
| Identity verification | Phone number | Account + roles | None | GoUNITY | GoUNITY |
| Privacy preserved | No | No | Yes (too much) | Yes | Yes |
| Self-hostable | Via Bot API | Via Bot API | N/A | Full | Full |
| E2E encrypted | No | No | Yes | Yes | Yes |
| Tamper-proof | No | No | No | No | **Yes (eFuse sealed)** |
| No-log provable | No | No | No | No | **Yes (sealed firmware)** |
| Physical attack surface | Cloud servers | Cloud servers | VPS | VPS | **Device only** |

GoBot is the first moderation bot that provides effective moderation, privacy preservation, **and** hardware-enforced tamper resistance. No other solution offers all three.

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
  <strong>GoBot - effective moderation for encrypted groups. No compromises on privacy. Hardware-enforced.</strong>
</p>
