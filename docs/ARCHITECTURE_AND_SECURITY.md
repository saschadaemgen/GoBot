# GoBot Architecture & Security

**Document version:** Season 2 | April 2026
**Component:** GoBot system (GoBot + GoKey + GoUNITY + GoLab)
**Copyright:** 2026 Sascha Daemgen, IT and More Systems, Recklinghausen
**License:** AGPL-3.0

---

## The idea in one sentence

A SimpleX group moderation bot where the server never sees a single message - and a community relay engine where thousands of users interact without the server knowing who they are.

---

## 1. System overview

### 1.1 Four components, four repos, one system

```
+------------------------------------------------------------------+
|                                                                  |
|  GoBot (Go service on VPS)           github.com/.../GoBot        |
|  Dumb Proxy + Community Relay        Go / Linux / systemd        |
|                                                                  |
|  - Holds all SMP/TLS connections                                 |
|  - Receives encrypted 16 KB blocks                               |
|  - Cannot decrypt anything (no keys)                             |
|  - Forwards blocks to GoKey via WSS                              |
|  - Receives signed command results                               |
|  - Executes commands (kick, ban, etc.)                           |
|  - Sends encrypted response blocks to SMP servers                |
|  - Buffers blocks when GoKey is temporarily offline              |
|  - Fans out GoLab community messages to channel subscribers      |
|                                                                  |
+---------------------------+--------------------------------------+
                            |
                     single WSS connection
                     (TLS 1.3 + mTLS)
                            |
+---------------------------v--------------------------------------+
|                                                                  |
|  GoKey (ESP32-S3 Crypto Engine)      SimpleGo Template           |
|  Secure Core                         C / FreeRTOS / ESP-IDF      |
|                                                                  |
|  - All private keys (eFuse + ATECC608B)                          |
|  - All ratchet state (encrypted NVS)                             |
|  - Decrypts messages (NaCl + Ratchet, 3-4 ms)                   |
|  - Parses: is this a bot command?                                |
|  - Signs command results (Ed25519 + replay protection)           |
|  - Encrypts bot responses as 16 KB blocks                       |
|  - Generates constant-size dummy blocks for non-commands         |
|  - Message plaintext NEVER leaves the ESP32                      |
|  - Hardware identity for GoLab (challenge-response)              |
|  - Stecker ziehen = Bot sofort tot                               |
|                                                                  |
|  Security: Secure Boot v2 (RSA-3072) | Flash Encrypt (AES-256)  |
|  JTAG disabled | ATECC608B | ChaCha20-Poly1305 (not HW-AES)     |
|                                                                  |
+------------------------------------------------------------------+

+------------------------------------------------------------------+
|                                                                  |
|  GoUNITY (Certificate Authority)     github.com/.../GoUNITY      |
|  Identity Server                     Go / Linux / systemd        |
|  Fork of smallstep/certificates      Apache-2.0 base             |
|                                                                  |
|  - Ed25519 certificate issuance                                  |
|  - CRL management and distribution                               |
|  - Challenge-response verification                               |
|  - Web frontend (registration, accounts)                         |
|  - HSM-backed signing key (YubiKey)                              |
|  - REST API for GoBot/GoKey                                      |
|  - Custom OID extensions for GoUNITY fields                      |
|                                                                  |
+------------------------------------------------------------------+

+------------------------------------------------------------------+
|                                                                  |
|  GoLab (Community Platform)          github.com/.../GoLab        |
|  Application Server + Browser Client Go + TypeScript             |
|                                                                  |
|  - Channel registry and configuration                            |
|  - Post persistence and search                                   |
|  - Activity stream aggregation                                   |
|  - Project management (issues, MRs, wikis)                       |
|  - Browser client built on simplex-js                            |
|  - Uses GoBot as relay, GoUNITY for identity                     |
|  - ActivityStreams 2.0 message format over SMP                   |
|                                                                  |
+------------------------------------------------------------------+
```

### 1.2 Why this split?

Every bot in an E2E encrypted group receives all messages in cleartext. The bot IS an endpoint. Transport encryption is irrelevant because the bot decrypts everything.

On a traditional VPS: SSH compromise = full group surveillance. The attacker copies the private key database and becomes the bot.

With GoBot + GoKey: The VPS has no keys. An attacker who compromises the server gets encrypted blocks they cannot read, signed commands they cannot forge, and a database with only queue IDs and server addresses. The private keys live on an ESP32-S3 at the user's home with Secure Boot, Flash Encryption, and permanently disabled JTAG. The only attack is physical access with laboratory equipment.

GoLab extends this model to communities: GoBot relays community messages (posts, reactions, follows) the same way it relays group messages - as encrypted blocks. In GoKey mode, not even GoBot can read the community content.

---

## 2. Data flow

### 2.1 Receiving a moderation command

```
1. User "Alice" types "!kick Bob" in SimpleX App

2. SimpleX encrypts (4 layers):
   Plaintext -> Ratchet -> NaCl Box -> NaCl Box -> TLS
   Sends to Alice's SMP server

3. GoBot (VPS) receives encrypted 16 KB block
   GoBot CANNOT read it (no keys)
   Forwards via WSS to GoKey:
   {"type": "block", "id": "uuid", "queue_id": "abc123", "payload": "<base64>"}

4. GoKey (ESP32) decrypts:
   Layer 3: simplex_secretbox_open()     ~0.3 ms
   Layer 2: simplex_secretbox_open()     ~0.3 ms
   Layer 1: smp_ratchet_decrypt()        ~1.5 ms
   Decompress: zstd_decompress()         ~0.5 ms
   Result: "!kick Bob"

5. GoKey parses: command detected
   Creates signed response:
   SIGN(seq=1742 || ts=1712234567 || grp=42 || hash=0xa3f1... || CMD:kick:Bob)
   Also creates encrypted bot reply "Bob was removed."
   Pads response to CONSTANT SIZE (anti-oracle)
   Adds RANDOM DELAY 100-500ms (anti-timing)

6. GoBot receives signed command + encrypted reply block
   Verifies Ed25519 signature
   Checks sequence number (reject if <= last seen)
   Executes kick via SMP protocol
   Sends encrypted reply block to correct SMP server
   GoBot never saw "!kick Bob" or "Bob was removed."
```

### 2.2 Non-command message

```
1. User "Charlie" writes "Hey, when do we meet?"

2. Block arrives at GoBot, forwarded to GoKey

3. GoKey decrypts: "Hey, when do we meet?"
   Not a command (no ! prefix)
   sodium_memzero() on plaintext buffer
   Creates DUMMY 16 KB encrypted block (constant-size response)
   Adds RANDOM DELAY (same range as real commands)
   Sends back to GoBot

4. GoBot receives response - IDENTICAL in size and timing
   Cannot distinguish this from a real command response
```

### 2.3 GoLab community post

```
1. User posts in GoLab channel #gochat-dev via browser client

2. Browser client creates ActivityStreams message:
   {"type": "Create", "object": {"type": "Note", "content": "..."}}
   Signs with Ed25519 key from GoUNITY certificate
   Encrypts via simplex-js, sends to SMP server

3. GoBot receives encrypted block on channel relay queue
   In GoKey mode: forwards to ESP32 for decryption
   GoKey decrypts, verifies signature, checks permissions
   GoKey returns: fan-out approved (signed)

4. GoBot fans out to all #gochat-dev subscriber queues
   Each subscriber has a unique SMP queue pair
   GoBot also forwards to GoLab App Server (for persistence)

5. Subscriber clients decrypt, verify Ed25519 signature
   Display: "CryptoNinja42: Fixed the memory leak..."
```

---

## 3. GoBot (VPS proxy)

### 3.1 What GoBot does

GoBot is the network-facing component. It runs as a Go service on a VPS, holds all SMP/TLS connections, and forwards encrypted blocks to GoKey for processing. GoBot is a dumb proxy - it cannot decrypt messages, forge commands, or access private keys.

| Property | Details |
|:---------|:--------|
| Language | Go |
| Deployment | Linux VPS, systemd service |
| SMP connections | Hundreds to thousands (Go goroutines) |
| GoKey connection | Single WSS with mTLS |
| GoUNITY connection | HTTPS REST API |
| GoLab connection | Internal API (localhost or mTLS) |
| Message access | NONE - only encrypted 16 KB blocks |
| Key material | NONE - all keys on GoKey (ESP32) |
| Database | SQLite (metadata only: queue IDs, server addresses) |
| Standalone mode | Optional - GoBot can run without GoKey (lower security) |

### 3.2 Primary mode (with GoKey)

```
SMP servers <--TLS--> GoBot <--WSS/mTLS--> GoKey (ESP32)

GoBot:
  1. Subscribes to SMP queues for all group members
  2. Receives encrypted 16 KB blocks
  3. Forwards blocks to GoKey via WSS
  4. Receives signed command results from GoKey
  5. Verifies Ed25519 signature + sequence number
  6. Executes command (SMP protocol: SEND, DEL, etc.)
  7. Receives encrypted response blocks from GoKey
  8. Sends response blocks to correct SMP server
  9. Buffers blocks when GoKey is temporarily offline
  10. Monitors GoKey heartbeat, alerts admin on failure
```

### 3.3 Standalone mode (without GoKey)

```
SMP servers <--TLS--> GoBot (decrypts locally)

GoBot:
  1. Holds all private keys locally (SQLite + SQLCipher)
  2. Decrypts messages, parses commands, encrypts responses
  3. All crypto happens on the VPS
  4. Lower security (~30-40% of SimpleX guarantees)
  5. Upgrade path: add GoKey later without reconfiguration
```

### 3.4 Community relay mode (GoLab)

```
GoLab clients <--SMP--> GoBot <--API--> GoLab App Server

GoBot:
  1. Manages SMP queue pairs for all channel subscribers
  2. Receives community messages (posts, reactions, follows)
  3. Verifies GoUNITY certificates and power levels
  4. Fans out to subscriber queues (O(n) per message)
  5. Forwards to GoLab App Server for persistence
  6. Enforces CRL (rejects revoked certificates)
  7. In GoKey mode: all of the above without seeing content
```

### 3.5 SMP frame-level client

GoBot does NOT implement a full SimpleX chat client. It operates at the SMP frame level:

| Operation | What GoBot does | What GoBot does NOT do |
|:----------|:---------------|:----------------------|
| TLS connections | Opens and maintains TLS 1.3 to SMP servers | - |
| SMP Subscribe | Sends SUB commands to receive messages | - |
| SMP Send | Wraps encrypted blocks in SEND commands | Encrypt/decrypt message content |
| SMP Ack | Acknowledges received messages | - |
| Double Ratchet | - | All ratchet operations (GoKey does this) |
| NaCl crypto | - | All NaCl encrypt/decrypt (GoKey does this) |
| Key management | - | No private keys (GoKey holds them) |

GoBot knows SMP framing (16 KB blocks, signatures, queue IDs) but not message content.

---

## 4. GoKey (ESP32-S3 crypto engine)

### 4.1 FreeRTOS task layout

| Task | Core | Stack | Role |
|:-----|:-----|:------|:-----|
| network_task | Core 0 | 16 KB SRAM | WiFi + WSS connection to VPS |
| gokey_task | Core 1 | 16 KB SRAM | Decrypt, parse, encrypt, sign |
| wifi_manager | Core 0 | 4 KB PSRAM | WiFi management, reconnect |

No display task loaded. Frees ~100 KB RAM (LVGL pool + draw buffers + task stack).

### 4.2 Crypto performance (ESP32-S3 at 240 MHz)

| Operation | Duration |
|:----------|:---------|
| NaCl crypto_box_open (Layer 2+3) | ~0.3 ms each |
| AES-256-GCM / ChaCha20 decrypt (Layer 1, 16 KB) | ~1.5 ms |
| Zstd decompress | ~0.5 ms |
| JSON parse | ~0.2 ms |
| Ed25519 sign (command result) | ~26 ms |
| **Total per message** | **~3-4 ms** (excluding signing) |

30 messages per minute = one every 2 seconds. GoKey needs 4 ms. 500x faster than needed.

### 4.3 eFuse security (irreversible after burn)

```
Secure Boot v2:       RSA-3072 signature check at every boot
Flash Encryption:     AES-256-XTS, key in eFuse, hardware-only
JTAG:                 Permanently disabled
UART Download:        Disabled
Direct Boot:          Disabled
```

### 4.4 ChaCha20-Poly1305 over AES-GCM

The ESP32-S3 hardware AES accelerator is vulnerable to side-channel power analysis (confirmed on ESP32-V3/C3/C6). GoKey uses ChaCha20-Poly1305 in software: 3x faster on ESP32-S3 (3.29 MB/s vs 1.13 MB/s), naturally constant-time, immune to power analysis.

For full GoKey hardware details, see [GoKey Architecture](https://github.com/saschadaemgen/SimpleGo/blob/main/templates/gokey/docs/ARCHITECTURE_AND_SECURITY.md).

---

## 5. GoUNITY (certificate verification)

### 5.1 Architecture

GoUNITY is a fork of smallstep/certificates (step-ca). Production-grade CA in Go. Ed25519 native. HSM support. Apache-2.0 license.

**What step-ca provides (not building ourselves):** Certificate signing, CRL, HSM integration (YubiKey), OIDC login, REST API, database backends, custom OID extensions, Docker deployment.

**What we build on top:** Web frontend (id.simplego.dev), account system, payment integration, challenge-response endpoint, GoKey CRL sync.

### 5.2 Verification flow

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

### 5.3 Certificate template

```json
{
  "subject": {"commonName": "{{.Insecure.User.username}}"},
  "extensions": [
    {"id": "1.3.6.1.4.1.XXXXX.1", "value": "{{.Insecure.User.username}}"},
    {"id": "1.3.6.1.4.1.XXXXX.2", "value": "{{.Insecure.User.level}}"}
  ],
  "keyUsage": ["digitalSignature"],
  "extKeyUsage": ["clientAuth"]
}
```

### 5.4 Ban enforcement

Bans linked to verified username. Rejoin with new profile -> must re-verify -> banned username rejected. New certificate requires new registration (costs money). Applies to both SimpleX groups and GoLab community channels.

For full GoUNITY details, see [GoUNITY Architecture](https://github.com/saschadaemgen/GoUNITY/blob/main/docs/ARCHITECTURE_AND_SECURITY.md).

---

## 6. GoLab integration

### 6.1 GoBot as community relay

GoLab uses GoBot as its relay engine. Instead of SimpleX's client-side fan-out (O(n^2) connections), GoBot handles centralized fan-out (O(n) connections):

```
Client-side fan-out (SimpleX groups):
  N members = N*(N-1)/2 queue pairs
  100 members = 4,950 queue pairs
  Each message sent N-1 times by the sender

GoBot relay fan-out (GoLab channels):
  N members = N queue pairs to GoBot
  100 members = 100 queue pairs
  Each message sent once, GoBot copies to N-1 queues
  Target: 10,000+ members per channel
```

### 6.2 ActivityStreams message handling

GoBot processes GoLab messages as ActivityStreams 2.0 objects. In GoKey mode, GoKey decrypts and validates the message, then tells GoBot what to do:

| ActivityStreams type | GoBot action |
|:--------------------|:-------------|
| Create + Note | Fan-out to channel subscribers |
| Announce | Fan-out repost to followers |
| Like | Fan-out reaction to channel |
| Follow | Establish new SMP queue pair |
| Block | Remove member, enforce ban |
| Remove | Delete content from channel |
| Update | Fan-out edit to channel |
| Add | Grant role to member |

### 6.3 Permission enforcement

GoBot checks permissions before relaying any GoLab message:

```
1. Verify sender's GoUNITY certificate (valid, not expired, not on CRL)
2. Check sender's power level for the target channel
3. Verify action is allowed at that power level
4. If GoKey mode: GoKey performs steps 1-3 and signs approval
5. GoBot executes fan-out only after verification passes
```

For full GoLab details, see [GoLab Architecture](https://github.com/saschadaemgen/GoLab/blob/main/docs/ARCHITECTURE_AND_SECURITY.md).

---

## 7. Security analysis

### 7.1 What we protect against

| Attack | Protection |
|:-------|:----------|
| VPS root compromise | No keys on server, only encrypted blocks |
| Private key theft | Keys in ESP32 eFuse + ATECC608B |
| Message surveillance | Plaintext never on server, not even in RAM |
| Command forgery | Ed25519 signed with replay protection |
| Traffic analysis (response oracle) | Constant-size, constant-time responses with dummy blocks |
| Command replay | Sequence numbers + timestamps + context binding |
| Man-in-the-middle (VPS-GoKey) | mTLS + certificate pinning |
| Firmware tampering | Secure Boot v2 (RSA-3072) |
| Flash readout | AES-256-XTS encryption |
| Debug access | JTAG permanently disabled via eFuse |
| AES side-channel | ChaCha20-Poly1305 in software (3x faster, constant-time) |
| Server forensics after seizure | Only encrypted blocks on disk, no keys, no plaintext |
| GoLab community surveillance | Same protections apply - GoBot relays encrypted blocks |

### 7.2 What we cannot protect against

| Attack | Why | Mitigation |
|:-------|:----|:-----------|
| Physical side-channel on ESP32 | Hardware AES vulnerable, ~60K power traces | ATECC608B for critical keys, ChaCha20 for bulk crypto |
| VPS drops/delays messages | VPS controls the network path | Heartbeat monitoring, sequence gap detection |
| VPS withholds signed commands | VPS can discard commands silently | Command acknowledgment protocol, admin alerts |
| Ratchet desync via message dropping | >1000 dropped messages breaks ratchet | Sequence monitoring, automatic re-key |
| Both VPS AND ESP32 compromised | Game over | Physical separation is the defense |

### 7.3 What a VPS attacker can do (damage potential)

| Attack | Impact | Detection |
|:-------|:-------|:----------|
| Drop all blocks | Bot/community goes silent | GoKey heartbeat timeout |
| Drop specific blocks | Targeted DoS on conversations | Sequence gap monitoring |
| Delay blocks | Slow moderation/community response | Timestamp checking |
| Withhold signed commands | Commands not executed | Command ack protocol |
| Replay signed commands | Rejected (sequence number) | Built into protocol |
| Forge commands | Rejected (invalid Ed25519 signature) | Built into protocol |
| Traffic analysis | Sees timing patterns | Constant-size responses from GoKey |

### 7.4 GoBot-specific hardening

| Measure | Purpose |
|:--------|:--------|
| mTLS with certificate pinning | Only THIS GoKey can connect |
| Ed25519 command verification | Cannot forge commands |
| Sequence numbers | Cannot replay commands |
| Command acknowledgments | Detects withheld commands |
| Heartbeat monitoring | Detects GoKey disconnection |
| Block buffering with TTL | Survives temporary GoKey offline |
| Minimal container (no shell) | Reduces post-compromise toolkit |
| iptables egress filtering | Prevents data exfiltration |
| Separate user (gobot:gobot) | Process isolation |

### 7.5 Security levels

```
Level 1: GoBot standalone (no GoKey)
  All keys on VPS. Simple. ~30-40% of SimpleX security.
  For: testing and non-sensitive groups/communities.

Level 2: GoBot + GoKey
  VPS is dumb proxy. Keys on ESP32 at home. ~85-90% security.
  For: production groups and communities where privacy matters.

Level 3: GoBot + GoKey + TEE (AMD SEV-SNP, future)
  Server in encrypted VM. Keys on ESP32. ~95% security.
  For: maximum protection.
```

### 7.6 Standalone mode security

When running without GoKey, GoBot holds all keys locally. This mode exists for users who want a quick bot without buying hardware.

| Threat | Standalone | With GoKey |
|:-------|:-----------|:-----------|
| VPS root compromise | Full message access | Only encrypted blocks |
| Key theft | Keys on disk (SQLCipher) | Keys in ESP32 eFuse |
| Message logging | Possible via code modification | Impossible (sealed firmware) |
| Server seizure | Full forensics possible | Only encrypted data |
| Command forgery | Possible with code access | Impossible (Ed25519 on ESP32) |

### 7.7 Known vulnerabilities

| ID | Severity | Description | Status |
|:---|:---------|:------------|:-------|
| GB-SEC-01 | HIGH | Standalone mode: all keys on disk | By design - GoKey upgrade resolves |
| GB-SEC-02 | MEDIUM | VPS can selectively drop messages | Mitigated by sequence monitoring |
| GB-SEC-03 | MEDIUM | VPS can withhold signed commands | Mitigated by ack protocol |
| GB-SEC-04 | LOW | Metadata visible (timing, queue IDs) | Mitigated by constant-size responses |
| GB-SEC-05 | LOW | GoLab channel membership visible as queue addresses | Mitigated by per-channel queue isolation |

---

## 8. GoKey Wire Protocol (summary)

The protocol between GoBot (VPS) and GoKey (ESP32) over WSS. Full specification: [GOKEY-WIRE-PROTOCOL.md](GOKEY-WIRE-PROTOCOL.md).

### Message types

| Message | Direction | Purpose |
|:--------|:----------|:--------|
| hello | GoKey -> GoBot | Handshake (version, pubkey, last seq) |
| welcome | GoBot -> GoKey | Handshake response |
| block | GoBot -> GoKey | Encrypted 16 KB SMP block |
| result | GoKey -> GoBot | Signed command or indistinguishable dummy |
| ping/pong | Both | Heartbeat every 30s |
| ack | Both | Message acknowledgment |
| error | GoKey -> GoBot | Signed error condition |

### Anti-analysis protections

Every response is 24 KB (PKCS#7 padded). Random delay 100-500 ms. Dummy blocks for non-commands. Signed errors. Monotonic sequence numbers. Timestamp validation (30s window). Block hash in signature.

### Signed command format

```
SIGN(seq_num || timestamp || group_id || block_hash || command)
```

GoBot verifies: valid signature, sequence > last seen, timestamp within 30 seconds, group ID matches context. Replay rejected. Forgery rejected.

---

## 9. Technology decisions

| Decision | Choice | Reason |
|:---------|:-------|:-------|
| GoBot language | Go | Same as GoRelay, handles thousands of connections, single binary |
| GoKey platform | ESP32-S3 via SimpleGo | Native SMP crypto stack proven (47 files, 21,863 LOC) |
| GoUNITY base | step-ca (smallstep) | Production-grade CA in Go, Ed25519 native, HSM support, Apache-2.0 |
| GoLab message format | ActivityStreams 2.0 | W3C standard, proven by Fediverse, extensible |
| Bulk crypto | ChaCha20-Poly1305 | 3x faster than AES-GCM on ESP32-S3, immune to power analysis |
| Command signing | Ed25519 | Fast (26 ms on ESP32), deterministic, proven secure |
| VPS-GoKey channel | WSS + mTLS | ESP32 initiates outbound (no public IP needed), NAT-friendly |
| Bot state storage | NVS Flash (GoKey) / SQLite (GoBot standalone) | Encrypted via eFuse key, ~88 KB usable |
| GoLab identity | DID:key (Ed25519) | W3C standard, self-describing, no resolution service |
| GoLab relay | GoBot fan-out | O(n) vs O(n^2), proven pattern (Cloudflare, banking) |

---

## 10. Deployment

### GoBot service

```bash
# Build
go build -o gobot ./cmd/gobot

# Configure
cp gobot.example.yaml gobot.yaml
# Edit: SMP servers, GoKey certificate paths, admin contact

# Run
./gobot --config gobot.yaml

# Or as systemd service
sudo cp gobot.service /etc/systemd/system/
sudo systemctl enable gobot
sudo systemctl start gobot
```

### systemd unit

```ini
[Unit]
Description=GoBot SimpleX Moderation Proxy + Community Relay
After=network.target

[Service]
Type=simple
User=gobot
Group=gobot
ExecStart=/opt/gobot/gobot --config /opt/gobot/gobot.yaml
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

### Dependencies

| Dependency | Version | Purpose |
|:-----------|:--------|:--------|
| Go | 1.24+ | Runtime |
| gorilla/websocket | latest | WSS server for GoKey |
| mattn/go-sqlite3 | latest | Metadata storage |
| crypto/ed25519 | stdlib | Command signature verification |
| crypto/tls | stdlib | mTLS for GoKey, TLS for SMP |

---

## 11. Comparable architectures

| System | Pattern | Scale |
|:-------|:--------|:------|
| Cloudflare Keyless SSL | Edge proxy + remote key server | ~8% of internet traffic |
| Qubes Split GPG | Network VM + crypto VM | Thousands of users |
| FIDO2/WebAuthn | Browser + hardware authenticator | Billions of devices |
| Hardware wallets (Ledger/Trezor) | Companion app + secure element | Millions of users |
| Apple Private Cloud Compute | OHTTP relay + Secure Enclave | Billions of devices |
| Banking HSM infrastructure | Payment terminal + HSM | Global financial system |
| **GoBot + GoKey** | **VPS proxy + ESP32 crypto engine** | **First for messaging bots** |

---

## 12. Related components

| Component | Role | Documentation |
|:----------|:-----|:-------------|
| [GoKey](https://github.com/saschadaemgen/SimpleGo) | Hardware crypto engine | [GoKey Architecture](https://github.com/saschadaemgen/SimpleGo/blob/main/templates/gokey/docs/ARCHITECTURE_AND_SECURITY.md) |
| [GoUNITY](https://github.com/saschadaemgen/GoUNITY) | Certificate authority | [GoUNITY Architecture](https://github.com/saschadaemgen/GoUNITY/blob/main/docs/ARCHITECTURE_AND_SECURITY.md) |
| [GoLab](https://github.com/saschadaemgen/GoLab) | Community platform | [GoLab Architecture](https://github.com/saschadaemgen/GoLab/blob/main/docs/ARCHITECTURE_AND_SECURITY.md) |
| [GoKey Wire Protocol](GOKEY-WIRE-PROTOCOL.md) | GoBot-GoKey communication | [Wire Protocol v0.2.0](GOKEY-WIRE-PROTOCOL.md) |
| [SimpleX Bot API Reference](SIMPLEX-BOT-API-REFERENCE.md) | SimpleX Chat Bot SDK types | [API Reference](SIMPLEX-BOT-API-REFERENCE.md) |

---

*GoBot Architecture & Security v2 - April 2026*
*IT and More Systems, Recklinghausen, Germany*

*"Your server holds the connections. Your hardware holds the keys. Nobody reads your messages."*
