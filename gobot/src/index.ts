// nwbot-v2/src/index.ts
// GoBot - SimpleX Chat Bot
// Uses the official simplex-chat Node.js SDK (native FFI, no CLI needed)

import { BOT_CONFIG } from "./config.js"
import { logger } from "./utils/logger.js"
import { checkRateLimit, cleanupRateLimits } from "./utils/rate-limiter.js"
import { dispatch, type CommandContext } from "./commands/index.js"

// Register commands (side-effect imports)
import "./commands/hilfe.js"   // !help
import "./commands/zeit.js"    // !time
import "./commands/datum.js"   // !date
import "./commands/status.js"  // !status
import "./commands/ping.js"    // !ping
import "./commands/mod.js"     // !kick !warn !members !warnings !clearwarn

// Types from the SDK
import { T } from "@simplex-chat/types"

// ---------------------------------------------------------------
// Extract command context from incoming chat items
// ---------------------------------------------------------------

function extractCommandContext(ci: T.AChatItem, content: T.MsgContent): CommandContext | null {
  if (content.type !== "text") return null

  const text = content.text.trim()
  if (!text) return null

  const chatInfo = ci.chatInfo
  const chatItem = ci.chatItem
  const chatDir = chatItem.chatDir

  let userId = "unknown"
  let userName = "Unknown"
  let chatType: "group" | "direct" = "direct"
  let groupId: number | undefined
  let groupName: string | undefined

  if (chatInfo.type === "group" && chatDir.type === "groupRcv") {
    chatType = "group"
    groupId = chatInfo.groupInfo.groupId
    groupName = chatInfo.groupInfo.groupProfile.displayName
    userId = String(chatDir.groupMember.memberId)
    userName = chatDir.groupMember.localDisplayName ?? "Unknown"
  } else if (chatInfo.type === "direct") {
    chatType = "direct"
    userId = String(chatInfo.contact.contactId)
    userName = chatInfo.contact.profile.displayName
  } else {
    return null
  }

  return {
    rawText: text,
    args: [],
    userId,
    userName,
    chatType,
    groupId,
    groupName,
  }
}

// ---------------------------------------------------------------
// Start bot
// ---------------------------------------------------------------

async function main() {
  logger.setLevel(BOT_CONFIG.logging.level as "debug" | "info")

  logger.info("========================================")
  logger.info("  GoBot v0.0.1-alpha - SimpleX Chat Bot")
  logger.info("========================================")
  logger.info(`Profile:  ${BOT_CONFIG.profile.displayName}`)
  logger.info(`Database: ${BOT_CONFIG.db.filePrefix}`)
  logger.info(`Loglevel: ${BOT_CONFIG.logging.level}`)
  logger.info("----------------------------------------")

  // Rate limit cleanup every 5 minutes
  setInterval(cleanupRateLimits, 5 * 60 * 1000)

  // Load SimpleX SDK
  const { bot } = await import("simplex-chat")

  logger.info("SimpleX SDK loaded, starting bot...")

  const [chat, user, address] = await bot.run({
    profile: {
      displayName: BOT_CONFIG.profile.displayName,
      fullName: BOT_CONFIG.profile.fullName,
    },
    dbOpts: {
      dbFilePrefix: BOT_CONFIG.db.filePrefix,
      dbKey: BOT_CONFIG.db.key,
    },
    options: {
      addressSettings: {
        welcomeMessage: { type: "text", text: BOT_CONFIG.welcomeMessage },
      },
    },

    onMessage: async (ci: T.AChatItem, content: T.MsgContent) => {
      try {
        const ctx = extractCommandContext(ci, content)
        if (!ctx) return

        logger.debug(
          `Message from ${ctx.userName} (${ctx.chatType}${ctx.groupName ? "/" + ctx.groupName : ""}): ${ctx.rawText}`
        )

        if (!checkRateLimit(ctx.userId)) {
          await chat.apiSendTextReply(ci, "Rate limit reached. Please wait.")
          return
        }

        const reply = await dispatch({ ...ctx, chatApi: chat, chatItem: ci })
        if (reply) {
          await chat.apiSendTextReply(ci, reply)
          logger.info(`Command executed: ${ctx.rawText} (by ${ctx.userName})`)
        }
      } catch (err) {
        logger.error(`Error processing message: ${err}`)
      }
    },

    // Auto-accept group invitations
    events: {
      receivedGroupInvitation: async (event) => {
        const groupName = event.groupInfo.groupProfile.displayName
        const groupId = event.groupInfo.groupId
        logger.info(`Group invitation received: "${groupName}" (id: ${groupId})`)
        try {
          await chat.apiJoinGroup(groupId)
          logger.info(`Joined group: "${groupName}"`)
        } catch (err) {
          logger.error(`Failed to join group "${groupName}": ${err}`)
        }
      },
    },
  })

  // Update profile with avatar
  if (BOT_CONFIG.profile.image) {
    try {
      const fullProfile = {
        ...user.profile,
        image: BOT_CONFIG.profile.image,
        preferences: user.profile.preferences ?? undefined,
      }
      await chat.apiUpdateProfile(user.userId, fullProfile)
      logger.info("Avatar set successfully")
    } catch (err) {
      logger.error("Failed to set avatar: " + err)
    }
  }

  logger.info("Bot is running!")
  logger.info(`User:    ${user.profile.displayName}`)
  if (address) {
    logger.info(`Address: ${JSON.stringify(address)}`)
  }
  logger.info("Waiting for messages...")
}

main().catch((err) => {
  logger.error(`Fatal error: ${err}`)
  process.exit(1)
})
