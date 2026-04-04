# GoKey Wire Protocol Specification

**Version:** 0.2.0-draft
**Status:** Draft
**Sprint:** 0 (Season 2)
**Scope:** GoBot (VPS) <-> GoKey (ESP32-S3) communication

---

## 1. Overview

This document defines the wire protocol between GoBot (Go service
on VPS) and GoKey (ESP32-S3 hardware crypto engine at home).

GoBot receives encrypted 16 KB SMP blocks from SimpleX servers
and forwards them to GoKey for decryption and command processing.
GoKey returns constant-size responses - either signed command
results or indistinguishable dummy blocks.

### 1.1 Design Goals

- **Constant-size frames:** Every frame on the wire is exactly
  the same size, regardless of content. Prevents traffic analysis.
- **Constant-time responses:** GoKey always responds within a
  fixed time window. No timing oracle.
- **Replay prevention:** Monotonic sequence numbers, timestamp
  windows, and context-bound signatures.
- **Crash recovery:** Acknowledgment system with block buffering
  on GoBot side.
- **Simplicity:** JSON payload, minimal state, clear error model.
- **Evolvability:** Protocol versioning from day one.

### 1.2 Transport

WebSocket Secure (WSS) with mutual TLS (mTLS).

- GoBot runs the WSS server (port TBD, default 6000).
- GoKey connects as WSS client with client certificate.
- Both certificates issued by GoUNITY CA (Season 4) or
  self-signed CA (Season 2-3).
- TLS 1.3 only. No fallback.
- Single persistent connection. Auto-reconnect on disconnect.

---

## 2. Frame Format

### 2.1 Wire Frame

Every WSS message is a single binary frame:

```
+------------------+-------------------+
| JSON Payload     | PKCS#7 Padding    |
| (variable)       | (to FRAME_SIZE)   |
+------------------+-------------------+
|<----------- FRAME_SIZE bytes -------->|
```

**FRAME_SIZE:** 24,576 bytes (24 KB)

A raw SMP block is 16,384 bytes (16 KB). Base64 encoding expands
this to approximately 21,848 bytes. Combined with JSON structure,
UUIDs, timestamps, and sender metadata, total payload reaches
roughly 22,500-23,000 bytes. The 24 KB frame size provides
sufficient headroom.

The JSON payload is UTF-8 encoded and padded with PKCS#7 to
exactly FRAME_SIZE bytes. The last byte of the frame always
indicates the number of padding bytes added.

### 2.2 Why PKCS#7

- Deterministic: receiver knows exactly how many bytes to strip.
- Self-describing: no separate length field needed.
- Standard: well-understood, no custom parsing.

### 2.3 Frame Processing

**Sender:**
1. Serialize message to JSON (UTF-8).
2. Calculate padding: `pad_len = FRAME_SIZE - len(json_bytes)`.
3. If `pad_len < 1`: ERROR - payload too large.
4. If `pad_len > 255`: use extended PKCS#7 (see 2.4).
5. Append padding bytes.
6. Send as single WSS binary message.

**Receiver:**
1. Receive WSS binary message.
2. Verify length is exactly FRAME_SIZE.
3. Read last byte to determine padding mode (see 2.4).
4. Strip padding, decode UTF-8, parse JSON.

### 2.4 Extended PKCS#7 for Large Padding

Standard PKCS#7 supports padding up to 255 bytes. Since small
messages (ping, pong, ack) need several KB of padding, we extend
the scheme:

- If `pad_len <= 255`: standard PKCS#7 (all bytes = pad_len).
- If `pad_len > 255`: last byte = 0x00 (signals extended mode),
  preceding 2 bytes = pad_len as big-endian uint16,
  all remaining padding bytes = 0x01.

**Extended padding structure:**
```
[0x01 0x01 ... 0x01] [pad_len_hi] [pad_len_lo] [0x00]
```

**Receiver detects mode by checking last byte:**
- Last byte > 0x00: standard PKCS#7, pad_len = last byte.
- Last byte = 0x00: extended mode, read preceding 2 bytes as
  big-endian uint16 for pad_len.

---

## 3. Protocol Versioning

### 3.1 Version Field

Every message includes a `"v"` field indicating the protocol
version. Current version: `1`.

```json
{
  "v": 1,
  "type": "...",
  ...
}
```

### 3.2 Version Negotiation

During the handshake (see 7.1), GoKey announces its supported
protocol version. GoBot confirms the version in its response.
If versions are incompatible, GoBot sends an error and closes
the connection.

### 3.3 Compatibility Rules

- Minor additions (new optional fields) do not require a version
  bump. Unknown fields are ignored.
- Structural changes (new message types, changed field semantics,
  removed fields) require a version bump.
- GoBot and GoKey must agree on the same major version.

---

## 4. Message Types

### 4.1 GoBot -> GoKey

#### 4.1.1 `block` - Encrypted SMP Block

Sent when GoBot receives an encrypted block from an SMP queue.

```json
{
  "v": 1,
  "type": "block",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "ts": "2026-04-04T12:00:00.000Z",
  "queue_id": "smp15.simplex.im#abc123",
  "group_id": "group_def456",
  "sender": {
    "member_id": "mem_789",
    "display_name": "Alice",
    "role": "member",
    "contact_id": "contact_012"
  },
  "payload": "<base64-encoded encrypted SMP block>"
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version (1) |
| type | string | yes | Always `"block"` |
| id | string | yes | UUIDv4, unique per message |
| ts | string | yes | ISO 8601 timestamp (GoBot clock) |
| queue_id | string | yes | SMP queue identifier |
| group_id | string | yes | SimpleX group ID |
| sender | object | yes | Sender metadata from SMP frame |
| sender.member_id | string | yes | Stable member ID |
| sender.display_name | string | yes | Current display name |
| sender.role | string | yes | owner/admin/moderator/member/observer |
| sender.contact_id | string | yes | Stable cross-group contact ID |
| payload | string | yes | Base64-encoded encrypted block |

#### 4.1.2 `ping` - Heartbeat

```json
{
  "v": 1,
  "type": "ping",
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "ts": "2026-04-04T12:00:30.000Z",
  "seq": 42
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"ping"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp |
| seq | integer | yes | Monotonic heartbeat counter |

**Interval:** Every 30 seconds.
**Timeout:** If no `pong` within 10 seconds, mark connection
as unhealthy. After 3 missed pongs, close and reconnect.

#### 4.1.3 `ack` - Acknowledgment

Sent after GoBot receives and validates a `result` from GoKey.

```json
{
  "v": 1,
  "type": "ack",
  "id": "550e8400-e29b-41d4-a716-446655440002",
  "ts": "2026-04-04T12:00:01.500Z",
  "ref_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"ack"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp |
| ref_id | string | yes | ID of the acknowledged message |

#### 4.1.4 `welcome` - Handshake Response

Sent by GoBot in response to GoKey's `hello` message.

```json
{
  "v": 1,
  "type": "welcome",
  "id": "550e8400-e29b-41d4-a716-446655440099",
  "ts": "2026-04-04T12:00:00.050Z",
  "ref_id": "660e8400-e29b-41d4-a716-446655440099",
  "accepted_version": 1,
  "server_time": "2026-04-04T12:00:00.050Z"
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"welcome"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp |
| ref_id | string | yes | ID of GoKey's `hello` |
| accepted_version | integer | yes | Negotiated protocol version |
| server_time | string | yes | GoBot clock for drift detection |

### 4.2 GoKey -> GoBot

#### 4.2.1 `hello` - Handshake Initiation

First message sent by GoKey after WSS/mTLS connection is
established.

```json
{
  "v": 1,
  "type": "hello",
  "id": "660e8400-e29b-41d4-a716-446655440099",
  "ts": "2026-04-04T12:00:00.010Z",
  "protocol_version": 1,
  "pubkey_fingerprint": "sha256:a1b2c3d4e5f6...",
  "last_seq": 1000,
  "firmware_version": "1.0.0"
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"hello"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp (GoKey clock) |
| protocol_version | integer | yes | Highest supported version |
| pubkey_fingerprint | string | yes | SHA-256 of Ed25519 public key |
| last_seq | integer | yes | Current sequence counter value |
| firmware_version | string | yes | GoKey firmware semver |

GoBot validates the public key fingerprint against its known
key. If mismatch, connection is rejected with an error.

GoBot checks `last_seq` against its own record. If GoBot's
last seen seq is higher than GoKey's `last_seq`, this indicates
possible NVS corruption or rollback attack - connection is
rejected with a security alert.

#### 4.2.2 `result` - Command Result or Dummy

Every `block` message receives exactly one `result` response.
If the decrypted block contains a command, the result carries
the signed action. If not, the result is an indistinguishable
dummy.

```json
{
  "v": 1,
  "type": "result",
  "id": "660e8400-e29b-41d4-a716-446655440000",
  "ts": "2026-04-04T12:00:00.350Z",
  "ref_id": "550e8400-e29b-41d4-a716-446655440000",
  "seq": 1001,
  "has_action": true,
  "action": {
    "command": "kick",
    "target_member_id": "mem_789",
    "group_id": "group_def456",
    "reply_text": "User removed for spam.",
    "block_hash": "sha256:abcdef1234567890..."
  },
  "signature": "<base64-encoded Ed25519 signature>"
}
```

**Dummy result (indistinguishable on the wire):**

```json
{
  "v": 1,
  "type": "result",
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "ts": "2026-04-04T12:00:00.420Z",
  "ref_id": "550e8400-e29b-41d4-a716-446655440003",
  "seq": 1002,
  "has_action": false,
  "action": {
    "command": "none",
    "target_member_id": "",
    "group_id": "",
    "reply_text": "",
    "block_hash": ""
  },
  "signature": "<base64-encoded Ed25519 signature over dummy>"
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"result"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp (GoKey clock) |
| ref_id | string | yes | ID of the corresponding `block` |
| seq | integer | yes | Monotonic sequence (GoKey counter) |
| has_action | boolean | yes | True if real command, false if dummy |
| action | object | yes | Always present, always same structure |
| action.command | string | yes | kick/ban/mute/block/warn/reply/none |
| action.target_member_id | string | yes | Target member (empty for dummy) |
| action.group_id | string | yes | Context group (empty for dummy) |
| action.reply_text | string | yes | Bot reply text (empty for dummy) |
| action.block_hash | string | yes | SHA-256 of original block (empty for dummy) |
| signature | string | yes | Ed25519 signature (see 5.1) |

**CRITICAL:** Both real and dummy results have identical structure
and identical field count. Padding ensures identical wire size.
GoKey adds random delay (100-500ms) before responding to prevent
timing analysis.

#### 4.2.3 `pong` - Heartbeat Response

```json
{
  "v": 1,
  "type": "pong",
  "id": "660e8400-e29b-41d4-a716-446655440002",
  "ts": "2026-04-04T12:00:30.100Z",
  "ref_id": "550e8400-e29b-41d4-a716-446655440001",
  "seq": 42
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"pong"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp |
| ref_id | string | yes | ID of the corresponding `ping` |
| seq | integer | yes | Echo of ping seq |

#### 4.2.4 `ack` - Acknowledgment

Sent after GoKey receives and validates a `block` from GoBot.

```json
{
  "v": 1,
  "type": "ack",
  "id": "660e8400-e29b-41d4-a716-446655440003",
  "ts": "2026-04-04T12:00:00.050Z",
  "ref_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

Same structure as GoBot ack (section 4.1.3).

#### 4.2.5 `error` - Signed Error Condition

```json
{
  "v": 1,
  "type": "error",
  "id": "660e8400-e29b-41d4-a716-446655440004",
  "ts": "2026-04-04T12:00:00.100Z",
  "ref_id": "550e8400-e29b-41d4-a716-446655440000",
  "seq": 1003,
  "code": "DECRYPT_FAILED",
  "message": "Failed to decrypt SMP block",
  "signature": "<base64-encoded Ed25519 signature>"
}
```

| Field | Type | Required | Description |
|:------|:-----|:---------|:------------|
| v | integer | yes | Protocol version |
| type | string | yes | Always `"error"` |
| id | string | yes | UUIDv4 |
| ts | string | yes | ISO 8601 timestamp |
| ref_id | string | yes | ID of the message that caused the error |
| seq | integer | yes | Monotonic sequence (increments like results) |
| code | string | yes | Error code (see 6.1) |
| message | string | yes | Human-readable description |
| signature | string | yes | Ed25519 signature (see 5.1) |

**Error messages are signed** to prevent a compromised VPS from
injecting spoofed errors that would cause GoBot to discard valid
command blocks. Error messages also increment the sequence
counter to maintain monotonic ordering. Error messages are
padded to FRAME_SIZE like all other messages.

---

## 5. Cryptographic Operations

### 5.1 Signature Format

GoKey signs every `result` and `error` with Ed25519.

**Signed data:** Canonical JSON of the signature payload object.

For `result` messages:
```json
{"block_hash":"sha256:abcdef...","command":"kick","group_id":"group_def456","ref_id":"550e8400-...","reply_text":"User removed for spam.","seq":1001,"target_member_id":"mem_789","ts":"2026-04-04T12:00:00.350Z","type":"result"}
```

For `error` messages:
```json
{"code":"DECRYPT_FAILED","message":"Failed to decrypt SMP block","ref_id":"550e8400-...","seq":1003,"ts":"2026-04-04T12:00:00.100Z","type":"error"}
```

**Canonical JSON rules:**
- Keys sorted alphabetically.
- No whitespace (no spaces, no newlines).
- No trailing commas.
- UTF-8 encoding.
- Strings use minimal escaping (only required escapes).

This approach eliminates delimiter injection risks and produces
a deterministic byte representation for signing.

GoBot verifies every signature. Invalid signatures are logged
and the result is discarded.

### 5.2 Sequence Numbers

- GoKey maintains a monotonic `seq` counter in NVS (non-volatile
  storage).
- Counter starts at 1 after provisioning.
- Counter increments by 1 for every `result` OR `error` sent.
- Counter NEVER decreases or resets.
- GoBot tracks last seen `seq`. Any `seq <= last_seen` is rejected.
- On GoKey reboot, counter resumes from NVS value.
- GoKey reports `last_seq` in its `hello` handshake for
  cross-validation.

### 5.3 Timestamp Validation

- GoBot checks: `|gobot_clock - result.ts| <= 30 seconds`.
- Requires loose time sync between VPS and ESP32.
- GoKey syncs time via NTP on startup and every 6 hours.
- NTP source: GoBot runs a local NTP relay on the VPS.
  GoKey syncs against this relay over the mTLS connection.
  Fallback: public NTP pool if relay is unreachable.
- If clocks drift beyond 30s, heartbeat pongs will also fail
  timestamp checks, triggering an alert.

### 5.4 Block Hash

- GoKey computes SHA-256 of the raw encrypted SMP block
  (before decryption).
- Included in signature to bind the command to a specific block.
- GoBot can independently compute the same hash to verify
  context binding.

### 5.5 Encryption

- Wire transport: TLS 1.3 (WSS/mTLS) - handled by transport.
- SMP block decryption: ChaCha20-Poly1305 (on GoKey only).
- No additional encryption layer on the wire protocol itself.
  TLS 1.3 is sufficient for the GoBot-GoKey link.

---

## 6. Error Handling

### 6.1 Error Codes

| Code | Origin | Description |
|:-----|:-------|:------------|
| DECRYPT_FAILED | GoKey | Cannot decrypt SMP block |
| INVALID_FORMAT | both | JSON parse error or missing fields |
| PAYLOAD_TOO_LARGE | both | Payload exceeds FRAME_SIZE |
| SEQ_MISMATCH | GoBot | Sequence number out of order |
| SIG_INVALID | GoBot | Ed25519 signature verification failed |
| TIMESTAMP_EXPIRED | GoBot | Timestamp outside 30s window |
| RATCHET_ERROR | GoKey | Double Ratchet state corruption |
| NVS_WRITE_FAILED | GoKey | Cannot persist sequence counter |
| UNKNOWN_QUEUE | GoKey | queue_id not in known queues |
| RATE_LIMITED | GoKey | Too many blocks per second |
| VERSION_MISMATCH | both | Incompatible protocol version |
| PUBKEY_MISMATCH | GoBot | Hello fingerprint does not match known key |
| SEQ_ROLLBACK | GoBot | Hello last_seq lower than expected |

### 6.2 Error Severity

**Critical (halt processing):**
- NVS_WRITE_FAILED: Sequence counter integrity at risk.
- SEQ_ROLLBACK: Possible tampering or NVS corruption.
- PUBKEY_MISMATCH: Wrong device or key compromise.

**Security (log + alert, discard message):**
- SEQ_MISMATCH, SIG_INVALID, TIMESTAMP_EXPIRED: Possible attack.

**Operational (log, continue):**
- DECRYPT_FAILED, RATCHET_ERROR: Queue-level issue.
- UNKNOWN_QUEUE, RATE_LIMITED: Transient conditions.
- INVALID_FORMAT, PAYLOAD_TOO_LARGE: Bug or corruption.

### 6.3 Error Recovery

**DECRYPT_FAILED:** Log, discard block. May indicate ratchet
desync - GoKey should attempt ratchet recovery if available.

**SEQ_MISMATCH / SIG_INVALID / TIMESTAMP_EXPIRED:** Log as
security event. Do NOT execute action. Alert operator.

**NVS_WRITE_FAILED:** Critical. GoKey must stop processing
and alert. Sequence counter integrity is non-negotiable.

**RATCHET_ERROR:** Log, attempt recovery. If recovery fails
for a queue, mark queue as dead and alert operator.

**VERSION_MISMATCH:** Close connection. Operator must update
the outdated component.

---

## 7. Connection Lifecycle

### 7.1 Handshake

```
GoKey                                GoBot
  |                                    |
  |--- [WSS/mTLS connect] ----------->|
  |                                    | (verify client cert)
  |                                    |
  |-- hello ------------------------->|
  |   (version, pubkey, last_seq)      |
  |                                    | (validate fingerprint)
  |                                    | (check seq consistency)
  |<-- welcome -----------------------|
  |   (accepted_version, server_time)  |
  |                                    |
  | Connection established.            |
```

GoKey MUST send `hello` within 5 seconds of WSS connection.
GoBot MUST respond with `welcome` or `error` within 5 seconds.
If either timeout is exceeded, the connection is closed.

### 7.2 Normal Operation

```
GoBot                              GoKey
  |                                  |
  |-- block (encrypted SMP) ------->|
  |                                  | (decrypt, check command)
  |                                  | (random delay 100-500ms)
  |<-- ack -------------------------|
  |<-- result (real or dummy) ------|
  |-- ack ------------------------->|
  |                                  |
  |-- ping (every 30s) ------------>|
  |<-- pong ------------------------|
  |                                  |
```

### 7.3 Reconnection

- GoKey initiates reconnection after disconnect.
- Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s max.
- On reconnect, full handshake (hello/welcome) is repeated.
- GoBot replays buffered blocks after successful handshake.
- GoKey verifies no sequence gaps occurred during downtime.

### 7.4 Shutdown

- Graceful: GoKey sends WSS close frame (code 1000).
- GoBot stops sending blocks, buffers incoming.
- Emergency: GoKey power loss ("Stecker ziehen") - no cleanup.
  GoBot detects via heartbeat timeout.

---

## 8. Block Buffering

GoBot buffers unacknowledged blocks in case GoKey disconnects.

### 8.1 Buffer Rules

- Maximum buffer size: 1000 blocks.
- Blocks are stored with their original timestamps.
- On reconnect, GoBot replays buffered blocks in order.
- Blocks older than 5 minutes are discarded (too stale to act on).
- Buffer is in-memory only (SQLite optional for persistence).

### 8.2 Flow Control

- GoBot sends blocks one at a time, waiting for `ack` before
  sending the next block.
- Exception: heartbeat `ping` can be sent at any time.
- If GoKey does not `ack` within 5 seconds, GoBot retransmits.
- Maximum 3 retransmits before marking block as failed.

---

## 9. Standalone Mode (Season 2 Testing)

When GoKey is not connected, GoBot operates in standalone mode
with reduced security.

In standalone mode:
- GoBot decrypts blocks locally (keys stored on VPS).
- No signature verification (no Ed25519 signing).
- No constant-time/constant-size guarantees.
- No sequence counter protection.
- Security estimate: ~30-40% of full system.

Standalone mode is for development and testing only. Production
deployments MUST use GoKey.

---

## 10. Time Synchronization

### 10.1 NTP Strategy

GoKey requires accurate time for timestamp validation.

**Primary:** GoBot runs a local NTP relay service on the VPS.
GoKey syncs against this relay. Since both communicate over
mTLS, the time source is authenticated.

**Fallback:** Public NTP pool (`pool.ntp.org`) if the VPS
relay is unreachable.

### 10.2 Sync Schedule

- On boot: mandatory NTP sync before sending `hello`.
- Every 6 hours: periodic resync.
- On heartbeat drift detection: immediate resync.

### 10.3 Drift Detection

If `|gobot_ts - gokey_ts|` in ping/pong exceeds 15 seconds,
GoBot logs a drift warning. At 25 seconds, GoBot sends an
alert to the operator. At 30 seconds, timestamp validation
starts rejecting messages.

---

## 11. Constants

```
FRAME_SIZE              = 24576     # bytes (24 KB)
PROTOCOL_VERSION        = 1        # current version
HEARTBEAT_INTERVAL      = 30       # seconds
HEARTBEAT_TIMEOUT       = 10       # seconds
MAX_MISSED_PONGS        = 3        # before reconnect
TIMESTAMP_WINDOW        = 30       # seconds
DRIFT_WARN_THRESHOLD    = 15       # seconds
DRIFT_ALERT_THRESHOLD   = 25       # seconds
RESPONSE_DELAY_MIN      = 100      # milliseconds
RESPONSE_DELAY_MAX      = 500      # milliseconds
BUFFER_MAX_BLOCKS       = 1000     # maximum buffered blocks
BUFFER_MAX_AGE          = 300      # seconds (5 minutes)
ACK_TIMEOUT             = 5        # seconds
MAX_RETRANSMITS         = 3        # per block
HANDSHAKE_TIMEOUT       = 5        # seconds
RECONNECT_BASE          = 1        # seconds
RECONNECT_MAX           = 30       # seconds
NTP_RESYNC_INTERVAL     = 21600    # seconds (6 hours)
WSS_PORT_DEFAULT        = 6000     # default WSS listen port
```

---

## 12. Open Questions

- [ ] Should GoKey support batch processing (multiple blocks per
  round-trip) or strictly one-at-a-time?
- [ ] Should the protocol support a `rotate_key` message type
  for periodic Ed25519 key refresh?
- [ ] Maximum `reply_text` length? Needs to fit within FRAME_SIZE
  after all other fields and base64 overhead.
- [ ] Should `hello` include a list of known queue_ids so GoBot
  can pre-filter irrelevant blocks?

---

*Draft v0.2.0 - Subject to review and iteration.*
