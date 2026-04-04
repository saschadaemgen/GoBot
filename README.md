<p align="center">
  <strong>GoBot</strong>
</p>

<p align="center">
  <strong>The world's first hardware-secured moderation bot for encrypted messaging.</strong><br>
  Your server holds the connections. Your hardware holds the keys. Nobody reads your messages.<br>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue.svg" alt="License"></a>
  <a href="#status"><img src="https://img.shields.io/badge/status-alpha-orange.svg" alt="Status"></a>
  <a href="https://github.com/saschadaemgen/SimpleGo"><img src="https://img.shields.io/badge/ecosystem-SimpleGo-green.svg" alt="SimpleGo"></a>
</p>

---

> "We use SimpleX - the most private messaging protocol on Earth. No user IDs, no metadata, double ratchet encryption, quantum-resistant, the whole nine yards. Then we invite a chatbot with admin rights that reads every message, lives on a Linux box with password 'changeme', and calls it 'group security'. That's like building a nuclear bunker and leaving the front door open because the pizza guy needs to get in."

GoBot is a moderation and verification bot for SimpleX Chat groups - but unlike every other bot on every other platform, **your server never sees a single message**.

GoBot splits the bot into two halves: a Go service on your VPS that holds network connections (but cannot decrypt anything), and an ESP32-S3 device at your home that holds all keys and performs all cryptography. Messages flow through your server as opaque 16 KB encrypted blocks. The ESP32 decrypts them, checks for bot commands, and sends back only the result - never the message text. The server is a dumb pipe. The hardware is the brain.

This architecture is based on proven security patterns used by Cloudflare Keyless SSL, Qubes Split GPG, FIDO2 hardware keys, and the global banking HSM infrastructure - applied for the first time to E2E encrypted messenger bots. Independent security analysis confirms the design is sound and novel enough to be publishable as an academic paper.

---

## How it works

```
[Your VPS]                              [Your home]
GoBot (Go service)                       GoKey (ESP32-S3)
Holds SMP connections                    Holds ALL private keys
Receives encrypted blocks                eFuse sealed firmware
Cannot decrypt anything                  Decrypts, checks for commands
                                         Message text stays here
     |                                        |
     |--- encrypted 16 KB block ---WSS/mTLS-->|
     |                                        |
     |                                   Decrypt (3-4 ms)
     |                                   Command? !kick Bob
     |                                        |
     |<-- "CMD:kick:Bob" (signed) -----------|
     |                                        |
     Executes kick                       Plaintext NEVER
     Never saw the message               leaves the ESP32

     Stecker ziehen = Bot sofort tot. Server ist eine leere Huelle.
```

**What a compromised server sees:** Encrypted blocks in, short signed command strings back. No message content. No private keys. No ratchet state. Nothing to steal.

**What it takes to break this:** Physical access to the ESP32 AND laboratory equipment for side-channel analysis. Not a password. Not an exploit. A soldering iron and an oscilloscope.

---

## Three components, three repos, one system

| Component | What it does | Where it runs | Repository |
|:----------|:-------------|:-------------|:-----------|
| **GoBot** | Dumb proxy - holds SMP connections, forwards encrypted blocks, executes commands | VPS (Go service) | [GoBot](https://github.com/saschadaemgen/GoBot) |
| **GoKey** | Secure core - holds all keys, decrypts/encrypts, parses commands, signs responses | ESP32-S3 at home | Template in [SimpleGo](https://github.com/saschadaemgen/SimpleGo) |
| **GoUNITY** | Identity - Ed25519 certificate authority for user verification and ban enforcement | VPS (Go service) | [GoUNITY](https://github.com/saschadaemgen/GoUNITY) (fork of step-ca) |

GoBot without GoKey works as a standalone bot on the VPS (lower security, ~30-40% of SimpleX guarantees). Adding GoKey raises security to ~85-90%. The hardware is optional but recommended.

---

## Security model

| Scenario | What the attacker gets |
|:---------|:----------------------|
| Attacker has root on VPS | Encrypted blocks they cannot read. Signed commands they cannot forge. |
| Attacker steals the server's hard drive | Encrypted database without keys. Worthless. |
| Attacker intercepts VPS-to-ESP32 traffic | mTLS encrypted. Cannot read or inject. |
| Attacker has the ESP32 device | eFuse-sealed firmware. Flash encrypted. JTAG disabled. Needs lab equipment. |
| Attacker has VPS AND ESP32 | Full compromise. This is the only scenario that breaks the system. |

### Security hardening (from independent analysis)

The architecture was reviewed against known attack patterns. Two critical issues were identified and their fixes are part of the design:

**Response Oracle Fix:** Every response from GoKey to GoBot is constant-size (padded to identical length), constant-time (identical code paths), and always produces a 16 KB outgoing dummy block - even for ignored messages. This prevents a compromised VPS from learning which messages trigger bot commands through size/timing analysis.

**Command Replay Fix:** Every signed command includes a monotonic sequence number, timestamp, group ID, and hash of the triggering message block. Signatures are unique and non-replayable. A compromised VPS cannot replay old commands.

**ChaCha20 over AES:** The ESP32-S3 hardware AES accelerator is vulnerable to side-channel power analysis (confirmed on ESP32-V3/C3/C6). GoKey uses ChaCha20-Poly1305 in software (3x faster on ESP32-S3, naturally constant-time, immune to power analysis).

---

## GoUNITY - identity verification

GoBot enforces [GoUNITY](https://github.com/saschadaemgen/GoUNITY) verified identities in SimpleX groups. GoUNITY is a fork of [smallstep/certificates](https://github.com/smallstep/certificates) (step-ca) - a production-grade certificate authority written in Go.

**Why this matters:** SimpleX has no persistent user identity. Banned users rejoin with a new profile in seconds. GoUNITY solves this with Ed25519 certificates bound to verified identities. Bans are linked to the certificate, not the SimpleX profile.

**Verification flow:**
1. User registers at id.simplego.dev (email + payment)
2. GoUNITY issues Ed25519 signed certificate
3. User sends certificate to GoBot via DM
4. GoKey verifies signature locally (no server contact)
5. GoKey sends challenge nonce
6. User signs nonce with private key (proves key ownership)
7. User is verified - no certificate sharing or replay possible

**What step-ca gives us for free:** Certificate signing, CRL generation, HSM integration (YubiKey), OIDC login, REST API, database backends, custom certificate templates with OID extensions. We build the web frontend and challenge-response logic on top.

---

## Current status

| Component | Status |
|:----------|:-------|
| GoBot Go service | Season 2 - planned |
| GoKey ESP32 firmware | Season 3 - planned (SimpleGo SMP stack proven) |
| GoUNITY certificate authority | Season 4 - repo forked, step-ca evaluating |
| GoKey Wire Protocol | Sprint 0 - designed, not implemented |
| Season 1 TypeScript prototype | Complete - API research done, findings documented |

**Season 1 achievements:** Complete SimpleX bot API research, all GroupMember types verified, 10 working commands, deployed prototype, security analysis of the bot paradox, Directory Bot research, hardware architecture designed and validated.

---

## Planned bot commands

**Admin commands** (require moderator/admin/owner role):

| Command | Action |
|:--------|:-------|
| !kick \<user\> | Remove member from group |
| !ban \<user\> \<reason\> | Ban by GoUNITY username (persistent) |
| !unban \<user\> | Remove ban |
| !mute \<user\> \<duration\> | Temporarily restrict to observer |
| !unmute \<user\> | Restore member role |
| !warn \<user\> | Issue tracked warning |
| !clearwarn \<user\> | Clear warnings |
| !banlist | Show active bans |
| !reports | Show pending user reports |
| !mode verified/mixed/open | Set group verification mode |

**User commands** (everyone):

| Command | Action |
|:--------|:-------|
| !help | Show available commands |
| !verify \<code\> | Submit GoUNITY verification |
| !report \<user\> \<reason\> | Report to admins |
| !mystatus | Check verification status |
| !rules | Show group rules |
| !ping | Check if bot is online |

---

## Roadmap

| Season | Focus | Status |
|:-------|:------|:-------|
| 1 | Research, prototype, API verification, architecture design | DONE |
| 2 | GoBot Go service, GoKey Wire Protocol, permission system | Planned |
| 3 | GoKey ESP32 firmware (SimpleGo template) | Planned |
| 4 | GoUNITY integration (step-ca, certificates, challenge-response) | Planned |
| 5 | Auto-moderation, multi-group, admin dashboard | Future |

---

## SimpleGo ecosystem

| Project | What it does |
|:--------|:-------------|
| [SimpleGo](https://github.com/saschadaemgen/SimpleGo) | Dedicated hardware messenger on ESP32-S3 - first native C implementation of SMP worldwide |
| [GoRelay](https://github.com/saschadaemgen/GoRelay) | Encrypted relay server (SMP + GRP) |
| [GoChat](https://github.com/saschadaemgen/GoChat) | Browser-native encrypted chat plugin |
| [GoShop](https://github.com/saschadaemgen/GoShop) | End-to-end encrypted e-commerce |
| [GoBot](https://github.com/saschadaemgen/GoBot) | Moderation bot (this project) |
| [GoKey](https://github.com/saschadaemgen/SimpleGo) | Hardware crypto engine for GoBot (SimpleGo template) |
| [GoUNITY](https://github.com/saschadaemgen/GoUNITY) | Certificate authority for identity verification (step-ca fork) |

---

## License

AGPL-3.0

---

<p align="center">
  <i>GoBot is part of the <a href="https://github.com/saschadaemgen/SimpleGo">SimpleGo ecosystem</a> by IT and More Systems, Recklinghausen, Germany.</i>
</p>

<p align="center">
  <strong>Your server holds the connections. Your hardware holds the keys. Nobody reads your messages.</strong>
</p>
