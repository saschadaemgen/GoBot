# GoBot - Season 1 Protocol

**Season:** 1
**Period:** March 31, 2026
**Status:** Complete
**Version at close:** v0.0.1-alpha

---

## Summary

Season 1 established the foundation for GoBot - a self-hosted moderation and verification bot for SimpleX Chat groups. The core bot framework was built using the official `simplex-chat` Node.js SDK (native FFI, no CLI dependency), deployed on a VPS alongside an existing SMP server, and tested with real users and groups. Extensive research was conducted into the SimpleX bot ecosystem, the platform's security model, and the fundamental privacy trade-offs of deploying bots in end-to-end encrypted groups.

---

## What was accomplished

### Research

- Comprehensive analysis of the SimpleX bot ecosystem: official SDKs (Haskell, TypeScript, Node.js), community frameworks (simpx-py, universal_bot, simploxide), and all known bot implementations
- Evaluated three approaches: CLI + WebSocket (legacy), TypeScript WebSocket client (stable), Node.js native SDK (beta). Chose the native SDK for zero CLI dependency
- Documented the complete SimpleX Bot API from `@simplex-chat/types` v0.3.0: GroupMember, GroupMemberRole, ChatInfo, CIDirection, MsgContent, AChatItem, and all verified ChatApi methods
- Researched the SimpleX Directory Bot's architecture and identified significant privacy implications (full message storage as architectural side effect, no audit coverage, no reproducible build verification)
- Analyzed the bot security paradox: any admin bot in an E2E group receives all messages in cleartext, making server compromise equivalent to full surveillance regardless of transport encryption layers
- Compared group discovery approaches across Signal (none), Matrix (federated directories, usually unencrypted), Session (communities without E2E), Telegram (full search, no E2E), and SimpleX (bot as admin member)

### Implementation

- **GoBot v0.0.1-alpha** built in TypeScript with the `simplex-chat` npm package (v6.5.0-beta.4.4)
- Modular command system with registry pattern (register + dispatch)
- Base commands: `!help`, `!time`, `!date`, `!status`, `!ping`
- Moderation commands: `!kick`, `!warn`, `!warnings`, `!clearwarn`, `!members`
- Auto-accept contact requests with configurable welcome message
- Auto-accept group invitations via `receivedGroupInvitation` event handler
- Per-user rate limiting with sliding window (configurable, default 10 req/60s)
- Profile avatar support via `apiUpdateProfile()` post-startup (SDK does not pass image during `bot.run()`)
- Deployed as systemd service (`gobot.service`) with auto-restart on the same VPS as the SMP server
- Bot successfully tested in group with multiple users

### Documentation

- `docs/SIMPLEX-BOT-API-REFERENCE.md` - verified API types, methods, events, and known limitations
- `docs/CONCEPT.md` - original project concept
- Updated `README.md` with current implementation status, architecture, and tech stack

---

## Key technical findings

### SimpleX Bot API (verified, not theoretical)

**GroupMember identification:**
- `groupMemberId` (number) - unique per group, assigned on join
- `memberId` (string) - internal identifier per group connection
- `memberContactId` (number, optional) - stable across groups IF user is also a bot contact
- `localDisplayName` (string) - user-controlled, NOT reliable for identification
- `memberProfile.profileId` (number) - profile-level identifier

**Role hierarchy (built into SimpleX, not custom):**
`owner > admin > moderator > member > author > observer`

Available via `memberRole` field on every `GroupMember` object. The `groupRcv` direction in incoming messages includes the full `GroupMember` of the sender, meaning every incoming group message carries the sender's role.

**Verified ChatApi methods:**

| Method | Tested |
|:-------|:-------|
| `apiSendTextReply(chatItem, text)` | YES |
| `apiListMembers(groupId)` | YES |
| `apiRemoveMembers(groupId, memberIds)` | NO |
| `apiJoinGroup(groupId)` | YES |
| `apiUpdateProfile(userId, profile)` | YES |
| `apiAcceptMember(groupId, memberId, role)` | NO |
| `apiBlockMembersForAll(groupId, memberIds)` | NO |
| `apiMembersRole(groupId, memberIds, role)` | NO |

### Avatar constraints

- Profile `image` field accepts data URI: `data:image/<type>;base64,<data>`
- Max total size: ~12,500 characters (protocol message limit is 15,610 bytes minus envelope overhead)
- Recommended: 192x192 JPEG, quality 60-75, resulting in ~3-5KB data URI
- PNG tends to exceed size limits
- `bot.run()` ignores the `image` field - must be set via `apiUpdateProfile()` after startup
- SimpleX apps display avatars as small thumbnails, high detail is wasted

### Bot security paradox

Any bot with admin rights in an E2E encrypted group receives all messages in cleartext. This is not a bug - it is an inherent property of the E2E model where the bot IS an endpoint. The implications:

- Transport encryption (SMP protocol, double ratchet, etc.) is irrelevant once the bot decrypts
- Server compromise = full group surveillance + moderation control
- Commercial hosted bots (Rose, Combot, etc.) centralize this risk across millions of groups
- SimpleX's own Directory Bot has this exact property, storing all messages as an architectural side effect
- No perfect solution exists. Mitigations: self-hosting only, minimal permissions, command-only interaction (no passive reading where possible)

### SimpleX group limitations

- Groups are fully decentralized: O(n) sends per message, O(n^2) total connections
- Practical limit ~100-200 members before connection fragmentation
- No ban persistence in the protocol - kicked users can rejoin with new profiles
- Member IDs are connection-scoped, not person-scoped - new profile = new identity
- `memberContactId` provides cross-group identification IF user maintains bot contact

---

## GoUNITY verification system (design analysis)

### Architecture as designed

GoUNITY is the identity verification layer that makes GoBot's moderation enforceable. Without it, bans are meaningless because SimpleX has no persistent user identity.

**Core flow:**
1. User registers a username at GoUNITY (id.simplego.dev)
2. GoUNITY issues an Ed25519-signed certificate binding the username to a keypair
3. User joins a GoBot-moderated SimpleX group
4. GoBot requests verification via DM
5. User sends certificate to GoBot
6. GoBot verifies Ed25519 signature locally (no GoUNITY server contact needed)
7. GoBot maps verified username to group member
8. Permissions granted based on verification level

**Key design principle:** GoUNITY server is NEVER contacted during verification. Offline cryptographic verification only. GoUNITY never learns which groups the user joins.

### Doorman concept

GoBot acts as a gatekeeper for groups:
- **Verified mode:** Only users with valid GoUNITY certificates can participate
- **Mixed mode:** Unverified users can participate with restrictions (message limits, no files, no links)
- **Open mode:** No verification required

### Ban enforcement via certificates

Traditional SimpleX bans are broken - users rejoin with new profiles. GoUNITY solves this:
- Ban is linked to verified username, not SimpleX member ID
- Banned user's certificate is rejected on re-entry regardless of profile
- New certificate requires new GoUNITY registration (costs money, requires verification)
- Certificate Revocation List (CRL) synced periodically from GoUNITY for revoked users

### Security analysis of the verification system

**Strengths:**
- Ed25519 signature verification is computationally trivial - scales to any group size
- Offline verification preserves privacy (no phone-home)
- Certificate cost creates economic barrier against ban evasion

**Identified weaknesses requiring Season 2+ solutions:**

1. **Certificate sharing:** Nothing prevents Alice from giving her certificate to Bob. Bob presents it as "Alice." Fix: Challenge-response protocol where GoBot sends a nonce and user must sign it with their private key. Requires user-side tooling.

2. **Replay attacks:** Certificate sent as text in group is visible to all members who could copy it. Fix: Verification must happen via DM only, never in group. Ideally with single-use tokens.

3. **No binding to SimpleX identity:** Certificate proves "this username exists at GoUNITY" but not "the person sending this is the keyholder." Fix: Challenge-response (see point 1).

4. **CRL timing window:** Between daily CRL updates, revoked users retain access (up to 24 hours). Acceptable for most cases but creates a window for determined attackers.

5. **Simpler alternative for Season 2:** One-time verification codes instead of full certificates. User gets code from website, sends to GoBot via DM, GoBot validates via single HTTPS call to GoUNITY, code is consumed. Eliminates sharing and replay. Trade-off: requires GoUNITY to be online during verification.

### Planned moderation commands (from README)

**Admin commands:**
- `/ban <user> <reason>` - Ban with reason (certificate-linked)
- `/mute <user> <duration>` - Temporary mute
- `/restrict <user> <rate>` - Rate limit specific user
- `/warn <user>` - Tracked warning
- `/unban`, `/unmute` - Remove restrictions
- `/status <user>` - Moderation history
- `/banlist` - Active bans
- `/reports` - Pending user reports
- `/mode verified|mixed|open` - Set group verification mode

**User commands:**
- `/verify <certificate>` - Submit GoUNITY certificate
- `/report <user> <reason>` - Report to admins
- `/mystatus` - Check own verification status
- `/rules` - Show group rules

**Auto-moderation (planned):**
- Spam detection (message frequency thresholds)
- Flood protection (max messages/hour)
- New member restrictions (read-only cooldown, no files/links initially)

---

## Known issues and technical debt

1. **Moderation commands have no permission checking.** Any user can `!kick` anyone. Must check `memberRole` before executing privileged commands.
2. **Warnings are stored in RAM only.** Bot restart = all warnings lost. Needs SQLite persistence.
3. **German filenames in English project:** `hilfe.ts`, `zeit.ts`, `datum.ts` should be `help.ts`, `time.ts`, `date.ts`
4. **Project folder structure needs cleanup.** GoUNITY has no home yet in the repo.
5. **Avatar not displaying in some cases.** JPEG format at 192x192 works, but the loading mechanism (reading from `data/avatar.b64` at startup) was fragile during development due to config file overwrites.
6. **No persistent ban list.** Kicks are immediate but not remembered across restarts.
7. **`bot.run()` does not support the profile `image` field.** Workaround via `apiUpdateProfile()` is functional but hacky.
8. **Untested API methods:** `apiRemoveMembers`, `apiBlockMembersForAll`, `apiMembersRole`, `apiAcceptMember` - types exist in SDK but have not been tested in production.

---

## Handover for Season 2

### Priority 1: Permission system
- Check `memberRole` on incoming commands before execution
- Only `admin`, `owner`, `moderator` can use `!kick`, `!warn`, `!clearwarn`
- All users can use `!help`, `!time`, `!date`, `!status`, `!ping`
- Store bot-specific admin list in config for cross-group super-admin capability

### Priority 2: Persistent storage
- Move warnings from RAM `Map` to SQLite
- Implement ban list with SQLite persistence
- Store member verification status (prep for GoUNITY)
- Survive bot restarts without losing moderation state

### Priority 3: GoUNITY integration
- Design the verification flow (one-time codes for Season 2, full certificates for later)
- Build the GoUNITY web service (registration, code generation, validation endpoint)
- Implement `/verify` command in GoBot
- Implement doorman flow (welcome + verification prompt on group join)
- Implement `/ban` with certificate-linked enforcement

### Priority 4: Project cleanup
- Rename `hilfe.ts` -> `help.ts`, `zeit.ts` -> `time.ts`, `datum.ts` -> `date.ts`
- Create `gounity/` folder in repo
- Clean up folder structure
- Update server path from `/opt/nwbot-v2` to `/opt/gobot` (partially done)

### Priority 5: Enhanced moderation
- `/mute` and `/unmute` commands
- `/restrict` for per-user rate limiting
- `/report` for user-to-admin reporting
- `/banlist` and `/status` commands
- New member cooldown (observer role for first N minutes)
- Shadow blocking via `apiBlockMembersForAll`

### Priority 6: Dashboard (Phase 2 from original plan)
- Web UI for bot configuration
- Live log streaming
- Group management interface
- Ban list management

---

## Infrastructure

| Component | Location | Status |
|:----------|:---------|:-------|
| GoBot source | `C:\Projects\GoBot\gobot\` | Active |
| GoBot deployed | `/opt/gobot/` on smp.simplego.dev | Running |
| GoBot service | `gobot.service` (systemd) | Enabled, auto-restart |
| GoBot database | `/opt/gobot/data/nwbot_chat.db` + `nwbot_agent.db` | Active |
| GoBot avatar | `/opt/gobot/data/avatar.b64` | 192x192 JPEG |
| GitHub repo | github.com/saschadaemgen/GoBot | Public |
| Node.js | v22.22.2 on server | Installed |
| SDK | simplex-chat v6.5.0-beta.4.4 | Installed |
| Types | @simplex-chat/types v0.3.0 | Installed |

**Current bot address:** `https://smp15.simplex.im/a#m36LEAL7T0hk9zqJ67i2cpzoiKkUbp3xMIeRQubKoRY`

**Warning:** Deleting the database files destroys the bot's identity, address, all contacts, and all group memberships. Never delete unless intentionally resetting.

---

## Lessons learned

1. **The SimpleX bot ecosystem is real but immature.** Documentation is sparse, the SDK is beta, and you will read source code more than docs.
2. **The native Node.js SDK is the right choice.** No CLI process management, no WebSocket reconnection logic, type-safe API. Worth the beta risk.
3. **Avatar handling is underdocumented.** The 12.5KB limit, the JPEG requirement, and the `bot.run()` image field being ignored cost hours to discover.
4. **Every bot in an E2E group is a privacy compromise.** This is not solvable with current technology. Self-hosting is the only meaningful mitigation. This insight should inform all GoBot marketing and documentation.
5. **SimpleX member IDs are connection-scoped, not person-scoped.** Without GoUNITY, effective bans are impossible. This validates the entire GoUNITY concept.
6. **The Season model works.** Small, focused increments with clear handover documentation prevent scope creep and knowledge loss.

---

*Season 1 closed March 31, 2026.*
*Season 2 scope: Permission system, persistent storage, GoUNITY integration, project cleanup.*
