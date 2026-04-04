# GoBot System Architecture

**Version:** Season 2 Concept (updated with security review findings)
**Date:** April 4, 2026
**Author:** Sascha Daemgen / IT and More Systems
**Status:** Approved concept, security reviewed

---

## The idea in one sentence

A SimpleX group moderation bot where the server never sees a single message.

---

## Three components, three repos, one system

```
+------------------------------------------------------------------+
|                                                                  |
|  GoBot (Go service on VPS)           github.com/.../GoBot        |
|  Dumb Proxy                          Go / Linux / systemd        |
|                                                                  |
|  - Holds all SMP/TLS connections                                 |
|  - Receives encrypted 16 KB blocks                               |
|  - Cannot decrypt anything (no keys)                             |
|  - Forwards blocks to GoKey via WSS                              |
|  - Receives signed command results                               |
|  - Executes commands (kick, ban, etc.)                           |
|  - Sends encrypted response blocks to SMP servers                |
|  - Buffers blocks when GoKey is temporarily offline              |
|                                                                  |
+---------------------------+--------------------------------------+
                            |
                     single WSS connection
                     (TLS 1.3 + mTLS + Noise_IK)
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
```

---

## Data flow

### Receiving a message

```
1. User "Alice" types "!kick Bob" in SimpleX App

2. SimpleX encrypts (4 layers):
   Plaintext -> Ratchet -> NaCl Box -> NaCl Box -> TLS
   Sends to Alice's SMP server

3. GoBot (VPS) receives encrypted 16 KB block
   GoBot CANNOT read it (no keys)
   Forwards via WSS to GoKey:
   {"from": "smp1.simplex.im", "queue": "abc123", "block": "<base64>"}

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

### Non-command message

```
1. User "Charlie" writes "Hey, when do we meet?"

2. Block arrives at GoBot, forwarded to GoKey

3. GoKey decrypts: "Hey, when do we meet?"
   Not a command (no ! prefix)
   sodium_memzero() on plaintext buffer
   Creates DUMMY 16 KB encrypted block (constant-size response)
   Adds RANDOM DELAY (same range as real commands)
   Sends back to GoBot

4. GoBot receives response - IDENTICAL in size and timing to a real command
   No outgoing SMP block needed (or: send dummy to random queue)
   GoBot cannot distinguish this from a real command response
```

---

## Security architecture

### What we protect against

| Attack | Protection |
|:-------|:----------|
| VPS root compromise | No keys on server, only encrypted blocks |
| Private key theft | Keys in ESP32 eFuse + ATECC608B |
| Message surveillance | Plaintext never on server, not even in RAM |
| Command forgery | Ed25519 signed with replay protection |
| Traffic analysis (response oracle) | Constant-size, constant-time responses with dummy blocks |
| Command replay | Sequence numbers + timestamps + context binding |
| Man-in-the-middle (VPS-GoKey) | mTLS + certificate pinning + optional Noise_IK |
| Firmware tampering | Secure Boot v2 (RSA-3072) |
| Flash readout | AES-256-XTS encryption |
| Debug access | JTAG permanently disabled via eFuse |
| AES side-channel | ChaCha20-Poly1305 in software (3x faster, constant-time) |
| Server forensics after seizure | Only encrypted blocks on disk, no keys, no plaintext |

### What we cannot protect against

| Attack | Why | Mitigation |
|:-------|:----|:-----------|
| Physical side-channel on ESP32 | Hardware AES vulnerable, ~60K power traces | ATECC608B for critical keys, ChaCha20 for bulk crypto |
| VPS drops/delays messages | VPS controls the network path | Heartbeat monitoring, sequence gap detection |
| VPS withholds signed commands | VPS can discard commands silently | Command acknowledgment protocol, admin alerts |
| Ratchet desync via message dropping | >1000 dropped messages breaks ratchet | Sequence monitoring, automatic re-key |
| Both VPS AND ESP32 compromised | Game over | Physical separation is the defense |

### Security levels

```
Level 1: GoBot standalone on VPS (no GoKey)
  All keys on server. Simple. ~30-40% of SimpleX security.
  For: people who want a quick bot.

Level 2: GoBot + GoKey
  Server is dumb proxy. Keys on ESP32 at home.
  ~85-90% of SimpleX security.
  For: people who are serious about privacy.

Level 3: GoBot + GoKey + TEE (AMD SEV-SNP, future)
  Server in encrypted VM. Keys on ESP32.
  ~95% of SimpleX security.
  For: maximum protection.
```

---

## GoKey Wire Protocol

The protocol between GoBot (VPS) and GoKey (ESP32) over WSS.

### VPS -> ESP32 (encrypted block delivery)

```json
{
  "type": "block",
  "seq": 174283,
  "from_server": "smp1.simplex.im",
  "queue_id": "abc123def456",
  "block": "<base64 encoded 16 KB encrypted SMP block>"
}
```

### ESP32 -> VPS (command response - CONSTANT SIZE)

```json
{
  "type": "response",
  "seq": 174283,
  "action": "CMD",
  "command": "kick",
  "target": "Bob",
  "group_id": 42,
  "reply_block": "<base64 encoded 16 KB encrypted bot reply>",
  "reply_server": "smp2.simplex.im",
  "reply_queue": "def456abc123",
  "signature": "<Ed25519 signature over seq||ts||grp||hash||cmd>",
  "timestamp": 1712234567,
  "block_hash": "0xa3f1..."
}
```

**For non-commands (IGNORE), the response is padded to IDENTICAL size:**

```json
{
  "type": "response",
  "seq": 174283,
  "action": "NOP",
  "command": "",
  "target": "",
  "group_id": 0,
  "reply_block": "<base64 encoded 16 KB DUMMY block>",
  "reply_server": "",
  "reply_queue": "",
  "signature": "<Ed25519 signature over seq||ts||0||hash||NOP>",
  "timestamp": 1712234567,
  "block_hash": "0xa3f1..."
}
```

Both responses are exactly the same size after JSON serialization + padding.

### Heartbeat

```json
{"type": "heartbeat", "seq": 174284, "timestamp": 1712234570}
```

ESP32 sends every 30 seconds. If GoBot misses 3 consecutive heartbeats, it alerts the admin via SimpleX DM.

### Command acknowledgment

```json
{"type": "ack", "seq": 174283, "executed": true}
```

GoBot confirms command execution. If GoKey doesn't receive ack within 10 seconds, it logs the gap. Persistent gaps trigger admin alert.

---

## GoUNITY verification

### What step-ca provides (not building ourselves)

- Ed25519 certificate signing and validation
- CRL generation and HTTPS distribution
- Custom OID extensions for GoUNITY fields (username, level)
- YubiKey/HSM integration for CA signing key
- OIDC provisioner (login via existing accounts)
- REST API for certificate lifecycle
- Database backends (BoltDB, Postgres, MySQL)
- Go templates for custom certificate formats

### What we build on top

- Web frontend for registration (id.simplego.dev)
- Account system (email verification, password)
- Payment integration (registration fee as anti-spam)
- Challenge-response endpoint
- GoKey CRL sync mechanism

### Certificate template

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

### Verification with challenge-response

```
1. User sends certificate to GoBot (DM only)
2. GoKey verifies Ed25519 signature (local, offline)
3. GoKey sends random nonce: "Sign this: 7a8f3c..."
4. User signs nonce with private key
5. GoKey verifies: signature matches public key in certificate
6. Proof: user holds the private key, not just the certificate text
7. Certificate sharing and replay are impossible
```

### Ban enforcement

Bans linked to verified username. Rejoin with new profile -> must re-verify -> banned username rejected. New certificate requires new registration (costs money).

---

## Technology decisions

| Decision | Choice | Reason |
|:---------|:-------|:-------|
| GoBot language | Go | Same as GoRelay, handles thousands of connections, single binary |
| GoKey platform | ESP32-S3 via SimpleGo | Native SMP crypto stack proven (47 files, 21,863 LOC) |
| GoUNITY base | step-ca (smallstep) | Production-grade CA in Go, Ed25519 native, HSM support, Apache-2.0 |
| Bulk crypto | ChaCha20-Poly1305 | 3x faster than AES-GCM on ESP32-S3, immune to power analysis |
| Command signing | Ed25519 | Fast (26 ms on ESP32), deterministic, proven secure |
| VPS-GoKey channel | WSS + mTLS | ESP32 initiates outbound (no public IP needed), NAT-friendly |
| Bot state storage | NVS Flash (GoKey) / SQLite (GoBot standalone) | Encrypted via eFuse key, ~88 KB usable |

---

## Season plan

| Season | Focus | Components |
|:-------|:------|:-----------|
| 1 | Research, prototype, API verification, architecture | GoBot (TS prototype) | 
| 2 | GoBot Go service, Wire Protocol, permission system | GoBot |
| 3 | GoKey ESP32 firmware (SimpleGo template) | GoKey |
| 4 | GoUNITY integration (step-ca, certificates, challenge-response) | GoUNITY + GoBot + GoKey |
| 5 | Auto-moderation, multi-group, admin dashboard | All |

---

## Comparable architectures

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

*GoBot System Architecture v2 - April 2026*
*IT and More Systems, Recklinghausen, Germany*

*"Your server holds the connections. Your hardware holds the keys. Nobody reads your messages."*
