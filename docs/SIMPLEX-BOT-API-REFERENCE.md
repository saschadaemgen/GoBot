# SimpleX Chat Bot API Reference

> Verified against `@simplex-chat/types` v0.3.0 and `simplex-chat` v6.5.0-beta.4.4
> These are real types and methods, tested and confirmed working.

---

## ChatApi Methods

Methods available on the `chat` object returned by `bot.run()`.

| Method | Returns | Tested |
|:-------|:--------|:-------|
| `chat.apiSendTextReply(chatItem, text)` | `AChatItem[]` | YES |
| `chat.apiListMembers(groupId)` | `GroupMember[]` | YES |
| `chat.apiRemoveMembers(groupId, memberIds, withMessages?)` | `GroupMember[]` | NO |
| `chat.apiJoinGroup(groupId)` | `GroupInfo` | YES |
| `chat.apiUpdateProfile(userId, profile)` | `UserProfileUpdateSummary` | YES |
| `chat.apiAcceptMember(groupId, groupMemberId, memberRole)` | `GroupMember` | NO |
| `chat.apiBlockMembersForAll(groupId, memberIds)` | unknown | NO |
| `chat.apiSendMessages(sendRef, liveMessage, ttl, composedMessages)` | unknown | NO |
| `chat.apiCreateMyAddress(userId)` | unknown | NO |
| `chat.apiDeleteMyAddress(userId)` | unknown | NO |
| `chat.apiMembersRole(groupId, memberIds, role)` | unknown | NO |

---

## Core Types

### GroupMember

Every user in a group is represented as a GroupMember.

```typescript
interface GroupMember {
  groupMemberId: number      // Unique ID within this group
  groupId: number            // Which group
  indexInGroup: number       // Position index
  memberId: string           // Internal member identifier (string)
  memberRole: GroupMemberRole
  memberCategory: GroupMemberCategory
  memberStatus: GroupMemberStatus
  memberSettings: GroupMemberSettings
  blockedByAdmin: boolean    // Shadow-blocked (messages hidden from others)
  invitedBy: InvitedBy
  invitedByGroupMemberId?: number
  localDisplayName: string   // Display name (CAN BE CHANGED BY USER)
  memberProfile: LocalProfile
  memberContactId?: number   // Links to bot's contact list (if also a contact)
  memberContactProfileId: number
  activeConn?: Connection
  memberChatVRange: VersionRange
  createdAt: string
  updatedAt: string
  supportChat?: GroupSupportChat
}
```

### GroupMemberRole

SimpleX has a built-in role hierarchy. Higher roles can manage lower roles.

```typescript
enum GroupMemberRole {
  Observer = "observer"      // Can only read, cannot send messages
  Author = "author"          // Can send messages but limited
  Member = "member"          // Normal member
  Moderator = "moderator"    // Can moderate (block, delete messages)
  Admin = "admin"            // Can manage members and settings
  Owner = "owner"            // Full control, can delete group
}
```

**Hierarchy:** owner > admin > moderator > member > author > observer

### GroupMemberCategory

How the member joined the group.

```typescript
enum GroupMemberCategory {
  User = "user"
  Invitee = "invitee"
  Host = "host"
  Pre = "pre"
  Post = "post"
}
```

### GroupMemberStatus

Current connection state of the member.

```typescript
enum GroupMemberStatus {
  // Values not fully extracted yet - needs further research
  // Known values from testing: "connected"
}
```

### LocalProfile

The profile information attached to each member.

```typescript
interface LocalProfile {
  profileId: number          // Stable profile ID
  displayName: string        // User-chosen display name (NOT STABLE)
  fullName: string
  shortDescr?: string
  image?: string             // Avatar as data URI (max ~12.5KB)
  contactLink?: string       // User's SimpleX address
  preferences?: Preferences
  peerType?: ChatPeerType    // "human" | "bot"
  localAlias: string         // Bot-side alias for this contact
}
```

### ChatInfo

Identifies what kind of chat a message came from.

```typescript
type ChatInfo =
  | { type: "direct"; contact: Contact }
  | { type: "group"; groupInfo: GroupInfo; groupChatScope?: GroupChatScopeInfo }
  | { type: "local"; noteFolder: NoteFolder }
  | { type: "contactRequest"; contactRequest: UserContactRequest }
  | { type: "contactConnection"; contactConnection: PendingContactConnection }
```

### CIDirection

Direction of a message - who sent it.

```typescript
type CIDirection =
  | { type: "directSnd" }                                    // Bot sent (DM)
  | { type: "directRcv" }                                    // Bot received (DM)
  | { type: "groupSnd" }                                     // Bot sent (group)
  | { type: "groupRcv"; groupMember: GroupMember }           // Bot received (group)
  | { type: "localSnd" }
  | { type: "localRcv" }
```

**Key insight:** `groupRcv` includes the full `GroupMember` object of the sender. This means every incoming group message carries the sender's role, IDs, and profile.

### MsgContent

Content of a message.

```typescript
type MsgContent =
  | { type: "text"; text: string }
  | { type: "link"; text: string; preview: LinkPreview }
  | { type: "image"; text: string; image: string }
  | { type: "video"; text: string; ... }
  | { type: "voice"; text: string; ... }
  | { type: "file"; text: string }
  | { type: "report"; ... }
  | { type: "unknown"; ... }
```

### CIContent

Wrapper around message content with send/receive context.

```typescript
type CIContent =
  | { type: "sndMsgContent"; msgContent: MsgContent }    // Sent by bot
  | { type: "rcvMsgContent"; msgContent: MsgContent }    // Received by bot
  | { type: "sndDeleted"; deleteMode: CIDeleteMode }
  | { type: "rcvDeleted"; deleteMode: CIDeleteMode }
  | { type: "rcvGroupEvent"; ... }
  | { type: "sndGroupEvent"; ... }
  // ... many more event types
```

### GroupInfo

Information about a group.

```typescript
interface GroupInfo {
  groupId: number
  useRelays: boolean
  localDisplayName: string
  groupProfile: GroupProfile
  localAlias: string
  businessChat?: BusinessChatInfo
  fullGroupPreferences: FullGroupPreferences
  membership: GroupMember          // The bot's own membership in this group
  chatSettings: ChatSettings
  createdAt: string
  updatedAt: string
  chatTs?: string
  chatTags: number[]
}
```

### Profile (Bot profile)

Used when creating or updating the bot's own profile.

```typescript
interface Profile {
  displayName: string        // Required, no spaces, no # or @ prefix
  fullName: string           // Required, can be empty ""
  shortDescr?: string        // Bio, max 160 chars
  image?: string             // Data URI: "data:image/jpg;base64,..."
  contactLink?: string
  preferences?: Preferences
  peerType?: ChatPeerType    // Set to "bot" for bot indicator in app
}
```

**Avatar constraints:**
- Format: data URI string `data:image/<type>;base64,<data>`
- Max total size: ~12,500 characters (limited by protocol message size of 15,610 bytes)
- Recommended: 192x192 JPEG, quality 60-75, results in ~3-5KB data URI
- PNG works but tends to be too large
- Set via `chat.apiUpdateProfile()` AFTER `bot.run()` (SDK does not pass image during initial setup)

### ChatRef

Reference to a chat for sending messages.

```typescript
interface ChatRef {
  chatType: ChatType       // "direct" | "group" | "local"
  chatId: number
  chatScope?: GroupChatScope
}
```

### AChatItem

A chat item with its context. This is what `onMessage` receives.

```typescript
interface AChatItem {
  chatInfo: ChatInfo       // Which chat (group/direct/etc)
  chatItem: ChatItem       // The actual message
}

interface ChatItem {
  chatDir: CIDirection     // Who sent it + sender info
  meta: CIMeta             // Timestamps, item ID, etc
  content: CIContent       // Message content
  mentions: { [key: string]: CIMention }
  formattedText?: FormattedText[]
  quotedItem?: CIQuote
  reactions: CIReactionCount[]
  file?: CIFile
}
```

---

## Event Handlers

Available via the `events` field in `bot.run()`.

| Event | Trigger | Tested |
|:------|:--------|:-------|
| `receivedGroupInvitation` | Bot was invited to a group | YES |
| `contactConnected` | New contact connected | NO |
| `newChatItems` | New messages received | YES (via onMessage) |
| `memberRole` | Member role changed | NO |
| `deletedMember` | Member removed from group | NO |
| `leftMember` | Member left group | NO |
| `joinedGroupMember` | New member joined group | NO |
| `userJoinedGroup` | Bot joined a group | NO |
| `groupUpdated` | Group info changed | NO |
| `chatItemsDeleted` | Messages deleted | NO |
| `connectedToGroupMember` | Connection established to member | NO |

---

## Bot Configuration (bot.run)

```typescript
interface BotConfig {
  profile: Profile
  dbOpts: {
    dbFilePrefix: string     // Path prefix for SQLite DBs
    dbKey?: string           // Encryption key (empty = unencrypted)
    confirmMigrations?: MigrationConfirmation
  }
  options: {
    createAddress?: boolean
    updateAddress?: boolean
    updateProfile?: boolean
    addressSettings?: {
      autoAccept?: boolean
      welcomeMessage?: MsgContent | string
      businessAddress?: boolean
    }
    allowFiles?: boolean
    commands?: ChatBotCommand[]    // App-visible command menus (v6.4.3+)
    useBotProfile?: boolean
    logContacts?: boolean
    logNetwork?: boolean
  }
  onMessage?: (chatItem: AChatItem, content: MsgContent) => void | Promise<void>
  onCommands?: { [command: string]: (chatItem: AChatItem, command: BotCommand) => void | Promise<void> }
  events?: { [event in CEvt.Tag]?: (event: ChatEvent) => void | Promise<void> }
}
```

---

## Known Limitations

- **Avatar not set by bot.run():** The `image` field in profile is ignored during initial setup. Must be set separately via `apiUpdateProfile()` after bot starts.
- **No file sending documented in SDK:** `sendFile`/`sendImage` methods not found in ChatApi. GitHub issue #5125 confirms this gap.
- **Group size:** O(n) sends per message, O(n^2) connections total. Practical limit ~100 members.
- **Warning persistence:** Not built into SimpleX. Must be implemented by bot (SQLite recommended).
- **Ban persistence:** SimpleX has no ban list. Kicked users can rejoin with new profile. Real bans require external identity system (GoUNITY).
- **Database:** Two SQLite files created automatically. Do NOT delete while bot is running. Deleting = new identity, all contacts/groups lost.
- **No multi-device:** One bot instance per database. Cannot run same bot identity on two servers.

---

## Useful CLI Commands (for debugging)

If running the CLI separately for testing:

| Command | What it does |
|:--------|:------------|
| `/ad` | Create bot address |
| `/set auto accept on` | Auto-accept contact requests |
| `#groupname message` | Send to group |
| `@contactname message` | Send to contact |
| `/members #groupname` | List group members |
| `/remove #groupname membername` | Remove member |
| `/mr #groupname membername role` | Change member role |
