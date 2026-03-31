# GoBot - Technical Concept
# Moderation and Automation Bot for SimpleX Groups

**Project:** GoBot - SimpleX Group Moderation Bot
**Author:** Sascha Daemgen / IT and More Systems
**Date:** 2026-03-31
**Status:** Concept Phase

---

## 1. Overview

GoBot is a headless SimpleX client that joins groups as an admin
member and provides moderation, verification, and automation services.
It communicates through the standard SMP protocol - fully E2E encrypted,
indistinguishable from a regular group member.

GoBot's primary role is enforcing GoUNITY verified identities in
SimpleX groups. It acts as the bridge between the GoUNITY certificate
system and the standard SimpleX app, enabling verified-only groups
without any client modifications.

---

## 2. System architecture

### 2.1 Component diagram

```
+---------------------------------------------------------------+
|                        GoBot Server                           |
|                                                               |
|  +------------------+  +------------------+  +--------------+ |
|  | Bot Engine       |  | Moderation       |  | GoUNITY      | |
|  |                  |  | Engine           |  | Verifier     | |
|  | Message parsing  |  | Ban/mute/warn    |  | Ed25519 sig  | |
|  | Command routing  |  | Auto-moderation  |  | CRL check    | |
|  | Event handling   |  | Report handling  |  | Level check  | |
|  +--------+---------+  +--------+---------+  +------+-------+ |
|           |                      |                   |         |
|  +--------v-----------------------------------------v-------+ |
|  |                  State Manager                           | |
|  |  Per-group: members, bans, mutes, warnings, config       | |
|  |  Storage: SQLite per group                                | |
|  +----------------------------+-----------------------------+ |
|                               |                               |
+-------------------------------+-------------------------------+
                                |
                    +-----------v-----------+
                    |  SimpleX CLI Adapter  |
                    |  JSON API over stdio  |
                    +-----------+-----------+
                                |
                    +-----------v-----------+
                    |  simplex-chat CLI     |
                    |  (headless mode)      |
                    +-----------+-----------+
                                |
                    +-----------v-----------+
                    |  SMP Network          |
                    |  (E2E encrypted)      |
                    +-----------------------+
```

### 2.2 SimpleX CLI integration

GoBot wraps the simplex-chat CLI process. Communication happens via
JSON over stdin/stdout in the CLI's API mode:

```go
// Start simplex-chat in API mode
cmd := exec.Command("simplex-chat", "-p", dataDir, "--api")
stdin, _ := cmd.StdinPipe()
stdout, _ := cmd.StdoutPipe()

// Send command
json.NewEncoder(stdin).Encode(APICommand{
    Cmd:    "apiSendTextMessage",
    Params: map[string]any{
        "type":    "group",
        "groupId": groupId,
        "text":    "Welcome! Please verify with /verify <certificate>",
    },
})

// Receive event
var event APIEvent
json.NewDecoder(stdout).Decode(&event)
```

### 2.3 Alternative: Direct SMP connection

For advanced deployments, GoBot can bypass the SimpleX CLI and connect
directly to SMP servers using Go's crypto libraries. This eliminates
the CLI dependency but requires implementing the SMP agent protocol.

Since GoChat (Season 9) has proven that a non-Haskell SMP client can
achieve full E2E encryption (X3DH + Double Ratchet), a Go
implementation is technically feasible. However, the CLI wrapper is
simpler for Phase 1.

---

## 3. GoUNITY certificate verification

### 3.1 Verification flow

```go
func (b *Bot) handleVerifyCommand(group, user, certBase64 string) {
    // 1. Decode certificate
    cert, err := DecodeCertificate(certBase64)
    if err != nil {
        b.reply(group, user, "Invalid certificate format.")
        return
    }
    
    // 2. Verify Ed25519 signature
    if !ed25519.Verify(b.gounityPubKey, cert.SignedData(), cert.Signature) {
        b.reply(group, user, "Certificate signature invalid.")
        return
    }
    
    // 3. Check expiration
    if time.Now().After(cert.ExpiresAt) {
        b.reply(group, user, "Certificate expired. Please renew at id.simplego.dev")
        return
    }
    
    // 4. Check CRL (revocation)
    if b.crl.IsRevoked(cert.Username) {
        b.reply(group, user, "This certificate has been revoked.")
        return
    }
    
    // 5. Check ban list
    if b.groups[group].IsBanned(cert.Username) {
        b.reply(group, user, "Username " + cert.Username + " is banned from this group.")
        b.removeFromGroup(group, user)
        return
    }
    
    // 6. Check minimum level
    if cert.Level < b.groups[group].MinLevel {
        b.reply(group, user, "This group requires verification level " + 
            b.groups[group].MinLevel.String() + " or higher.")
        return
    }
    
    // 7. Success
    b.groups[group].SetVerified(user, cert.Username, cert.Level)
    b.reply(group, user, "Verified! Welcome, " + cert.Username + ".")
}
```

### 3.2 Certificate caching

Once a user is verified, GoBot stores the mapping locally:

```
SQLite: group_members table
  group_id    TEXT
  simplex_id  TEXT     (SimpleX internal member ID)
  username    TEXT     (GoUNITY verified name)
  level       INTEGER (verification level)
  verified_at TIMESTAMP
  cert_hash   TEXT    (SHA-256 of certificate, for change detection)
```

GoBot does NOT store the full certificate or any SimpleX queue
addresses. The `simplex_id` is the group-internal member identifier
that SimpleX assigns - it cannot be used to find the user on other
groups or the SMP network.

---

## 4. Moderation engine

### 4.1 Command processing

```go
type Command struct {
    Name   string   // "ban", "mute", "warn", etc.
    Target string   // username
    Args   []string // duration, reason, etc.
    Admin  string   // who issued the command
    Group  string   // which group
}

func (b *Bot) processCommand(cmd Command) {
    // Check: is sender an admin?
    if !b.groups[cmd.Group].IsAdmin(cmd.Admin) {
        b.reply(cmd.Group, cmd.Admin, "You don't have permission.")
        return
    }
    
    switch cmd.Name {
    case "ban":
        b.executeBan(cmd)
    case "mute":
        b.executeMute(cmd)
    case "restrict":
        b.executeRestrict(cmd)
    case "warn":
        b.executeWarn(cmd)
    case "unban":
        b.executeUnban(cmd)
    case "unmute":
        b.executeUnmute(cmd)
    case "banlist":
        b.showBanList(cmd)
    case "reports":
        b.showReports(cmd)
    case "mode":
        b.setGroupMode(cmd)
    case "status":
        b.showUserStatus(cmd)
    }
}
```

### 4.2 Auto-moderation rules

```go
type AutoModConfig struct {
    SpamDetection     bool
    FloodProtection   bool
    NewMemberCooldown time.Duration
    MaxMessagesPerHour int
    MaxImagesPerHour   int
    SlowMode          time.Duration // 0 = off
    AutoBanAfterMutes int           // 0 = off
}

func (b *Bot) checkAutoMod(group, user, message string) Action {
    state := b.groups[group].GetMemberState(user)
    
    // Flood protection
    if state.MessagesLastHour >= b.config.MaxMessagesPerHour {
        return AutoMute{Duration: 1 * time.Hour, Reason: "flood protection"}
    }
    
    // New member cooldown
    if time.Since(state.JoinedAt) < b.config.NewMemberCooldown {
        return Reject{Reason: "new member cooldown"}
    }
    
    // Slow mode
    if b.config.SlowMode > 0 && time.Since(state.LastMessage) < b.config.SlowMode {
        return Reject{Reason: "slow mode"}
    }
    
    // Spam pattern detection
    if isSpamPattern(message) {
        state.SpamScore++
        if state.SpamScore >= 3 {
            return AutoMute{Duration: 24 * time.Hour, Reason: "spam detected"}
        }
    }
    
    return Allow{}
}
```

### 4.3 State persistence

Each group's moderation state is stored in a local SQLite database:

```sql
-- Per group database: data/{group_id}.db

CREATE TABLE members (
    simplex_id      TEXT PRIMARY KEY,
    username        TEXT,              -- GoUNITY verified name (NULL if unverified)
    level           INTEGER DEFAULT 0,
    joined_at       TIMESTAMP,
    verified_at     TIMESTAMP,
    message_count   INTEGER DEFAULT 0,
    last_message_at TIMESTAMP
);

CREATE TABLE bans (
    username    TEXT PRIMARY KEY,
    reason      TEXT,
    banned_by   TEXT,
    banned_at   TIMESTAMP
);

CREATE TABLE mutes (
    username    TEXT PRIMARY KEY,
    until       TIMESTAMP,
    reason      TEXT,
    muted_by    TEXT
);

CREATE TABLE warnings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    username    TEXT,
    reason      TEXT,
    warned_by   TEXT,
    warned_at   TIMESTAMP
);

CREATE TABLE reports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    reported    TEXT,
    reporter    TEXT,
    reason      TEXT,
    message     TEXT,
    reported_at TIMESTAMP,
    status      TEXT DEFAULT 'pending'  -- pending, reviewed, dismissed
);

CREATE TABLE config (
    key         TEXT PRIMARY KEY,
    value       TEXT
);
```

---

## 5. Multi-group management

### 5.1 Group isolation

Each group runs as an independent moderation context. Ban lists,
configurations, and member states are completely separate. A ban in
Group A does not affect Group B unless the admin explicitly enables
shared ban lists.

### 5.2 Configuration

```yaml
# gobot.yaml - main configuration

bot:
  data_dir: "./data"
  simplex_cli: "/usr/local/bin/simplex-chat"
  log_level: "info"

gounity:
  pubkey: "base64(GoUNITY Ed25519 public key)"
  crl_url: "https://id.simplego.dev/v1/crl"
  crl_refresh: "24h"

groups:
  - id: "group-uuid-1"
    name: "SimpleGo Community"
    mode: "verified"           # open | mixed | verified | invite
    min_level: 1               # 1=Basic, 2=Verified, 3=Business, 4=Premium
    admins:
      - "Sascha"
      - "Moderator1"
    auto_moderation:
      spam_detection: true
      flood_protection: true
      max_messages_per_hour: 30
      max_images_per_hour: 5
      new_member_cooldown: "5m"
      slow_mode: "0s"
      auto_ban_after_mutes: 3
    unverified_restrictions:
      messages_per_hour: 5
      send_files: false
      send_links: false
      send_images: false

  - id: "group-uuid-2"
    name: "Casual Chat"
    mode: "mixed"
    admins:
      - "Sascha"
    auto_moderation:
      spam_detection: true
      flood_protection: true
      max_messages_per_hour: 60
```

---

## 6. GoShop integration (future)

GoBot can serve as the verification backend for GoShop transactions:

```
Customer in GoShop group:
  -> GoBot: /verify <certificate>
  -> GoBot: "Verified as MeinPrinz (Verified level)"
  
Customer: "I'd like to order the ESP32 board"
  -> GoBot checks: is customer verified? What level?
  -> GoBot to shop owner: "Order from MeinPrinz (Verified)"
  
Shop owner: /trust MeinPrinz
  -> GoBot stores trust relationship
  -> Future orders from MeinPrinz auto-approved
```

---

## 7. Security considerations

### 7.1 GoBot sees messages

As a group member, GoBot can read all messages in groups it joins.
This is necessary for moderation (spam detection, command processing).

**Mitigations:**
- GoBot does not store message content (only metadata for moderation)
- GoBot runs on the group admin's own server
- GoBot's database contains only usernames and moderation actions
- No message content is ever sent to GoUNITY or any external service

### 7.2 GoBot as attack target

A compromised GoBot could:
- Read group messages (same as any compromised member)
- Issue false bans/mutes
- Impersonate the admin

**Mitigations:**
- GoBot runs on admin's own infrastructure (self-hosted)
- Admin commands require GoUNITY-verified admin username
- All moderation actions are logged locally
- GoBot can be removed from a group instantly by any admin

### 7.3 SimpleX CLI trust

GoBot trusts the simplex-chat CLI binary. A compromised CLI could
feed GoBot false data. This is the same trust boundary as any
SimpleX user running the official client.

---

## 8. Roadmap

### Phase 1: Core bot
- SimpleX CLI integration (JSON API)
- Message parsing and event handling
- Basic command framework
- Single-group operation
- SQLite state storage

### Phase 2: GoUNITY integration
- Certificate verification (Ed25519)
- Ban enforcement by username
- CRL synchronization
- Verification level checking
- Doorman flow (welcome + verify)

### Phase 3: Moderation commands
- /ban, /mute, /restrict, /warn, /unban, /unmute
- /banlist, /reports, /status
- Report system (user reports)
- Moderation action logging

### Phase 4: Auto-moderation
- Spam pattern detection
- Flood protection (rate limiting)
- New member cooldown
- Slow mode
- Auto-escalation (mute -> ban)

### Phase 5: Multi-group
- Multi-group configuration (YAML)
- Per-group isolation
- Shared ban lists (opt-in)
- Admin dashboard (CLI)

### Phase 6: Web dashboard
- Web UI for bot configuration
- Real-time group monitoring
- Moderation queue
- Analytics (message counts, member growth)

### Phase 7: Advanced
- GoShop transaction verification
- Cross-group reputation
- Custom auto-mod rules (regex, ML)
- Plugin system for extensions

---

## 9. Open questions

1. **simplex-chat CLI API stability:** Is the JSON API mode stable
   enough for production use? What happens on CLI updates?

2. **Rate limits:** Does the SMP server impose rate limits on bot
   accounts? Can GoBot handle 100+ messages per minute?

3. **Group admin API:** Can GoBot programmatically set member roles
   and permissions via the CLI? Or only through chat commands?

4. **Multi-device:** Can GoBot maintain group membership if the
   server restarts? Does simplex-chat persist state across restarts?

5. **Certificate transport:** What's the best UX for users to submit
   certificates? Direct paste? QR code? Deep link?

---

*GoBot Technical Concept v1 - March 2026*
*IT and More Systems, Recklinghausen, Germany*
