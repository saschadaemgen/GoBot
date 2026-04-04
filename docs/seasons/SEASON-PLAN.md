# Season Plan

**Current Season:** 2
**Current Sprint:** 1
**Last Updated:** April 4, 2026

---

## Sprint 0: GoKey Wire Protocol Specification - COMPLETE

**Deliverable:** `docs/GOKEY-WIRE-PROTOCOL.md`

- [x] Frame format (24 KB constant-size, PKCS#7 + extended)
- [x] Protocol versioning (`"v": 1` in all messages)
- [x] Handshake (`hello` / `welcome`)
- [x] Message types: block, result, ping/pong, ack, error
- [x] Signature format (canonical JSON, Ed25519)
- [x] Sequence number design (monotonic, NVS-backed)
- [x] Timestamp validation (30s window)
- [x] Response Oracle (constant-size dummy blocks)
- [x] Block buffering strategy
- [x] Error codes and severity classification
- [x] Signed errors (prevent spoofed error injection)
- [x] NTP sync strategy (VPS relay + 6h resync)
- [x] Resolve open questions
- [x] Final review

---

## Sprint 1: GoBot Go Project Setup - IN PROGRESS

- [x] `go mod init` with module path
- [x] Project directory structure (`cmd/`, `internal/`)
- [x] Makefile (build, test, lint, run)
- [x] GitHub Actions CI (build + test + lint)
- [x] Basic logging framework
- [x] Configuration management (env vars)
- [x] README update (Go instructions, TS references removed)
- [x] .gitignore for Go
- [x] First passing test (5 config tests)

---

## Sprint 2: GoBot SMP Proxy

- [ ] SMP/TLS client (connect to SMP servers)
- [ ] Queue subscription management
- [ ] Encrypted block reception (raw 16 KB frames)
- [ ] Block forwarding interface (prep for GoKey connection)
- [ ] Connection health monitoring
- [ ] Reconnection with exponential backoff
- [ ] Integration test with live SMP server

---

## Sprint 3: GoBot Standalone Mode

- [ ] Local decryption (ChaCha20-Poly1305)
- [ ] Double Ratchet state management
- [ ] Command parser (bot command syntax)
- [ ] Permission system (memberRole checks)
- [ ] Command execution via SimpleX API
  - [ ] apiSendTextReply (bot responses)
  - [ ] apiRemoveMembers (kick)
  - [ ] apiBlockMembersForAll (shadow block)
  - [ ] apiMembersRole (mute/set observer)
- [ ] Persistent warnings and bans (SQLite)
- [ ] Bot profile management (avatar, bot indicator)
- [ ] Group join handling (apiJoinGroup, apiAcceptMember)

---

## Sprint 4: GoBot WSS Server for GoKey

- [ ] WSS server (TLS 1.3, mTLS)
- [ ] Self-signed CA for Season 2-3 certificates
- [ ] Hello/Welcome handshake implementation
- [ ] Block forwarding to GoKey
- [ ] Result reception and signature verification
- [ ] Dummy block handling (discard silently)
- [ ] Sequence number tracking and validation
- [ ] Timestamp validation
- [ ] Block hash verification (context binding)
- [ ] Block buffering (1000 blocks, 5 min max age)
- [ ] Flow control (one block at a time, ack-based)
- [ ] Heartbeat monitoring (ping/pong)
- [ ] Reconnection handling
- [ ] NTP relay service
- [ ] Fallback to standalone mode when GoKey offline

---

## Sprint 5: Documentation + Season Close

- [ ] Season 2 Protocol document
- [ ] Season 3 Handoff document
- [ ] Update SYSTEM-ARCHITECTURE.md if needed
- [ ] Update ARCHITECTURE_AND_SECURITY.md if needed
- [ ] Code review and cleanup
- [ ] Final test pass

---

*Living document - updated as sprints progress.*
