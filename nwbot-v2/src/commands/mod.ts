import { registerCommand } from "./index.js"
import { logger } from "../utils/logger.js"

const warnings = new Map<string, number>()
const MAX_WARNINGS = 3

registerCommand({
  name: "members",
  description: "List group members",
  groupOnly: true,
  handler: async (ctx) => {
    if (!ctx.chatApi || !ctx.groupId) return "No API access."
    try {
      const resp = await ctx.chatApi.apiListMembers(ctx.groupId)
      if (!resp || !Array.isArray(resp)) return "Could not retrieve member list."
      const lines = resp.map((m: any) => {
        const name = m.localDisplayName || m.memberProfile?.displayName || "?"
        const role = m.memberRole || "?"
        const status = m.memberStatus || "?"
        return `  ${name} (${role}, ${status})`
      })
      return [`Members in ${ctx.groupName || "group"} (${lines.length}):`, "", ...lines].join("\n")
    } catch (err) {
      logger.error(`Error in !members: ${err}`)
      return "Error retrieving member list."
    }
  },
})

registerCommand({
  name: "kick",
  description: "Remove a member from the group",
  groupOnly: true,
  handler: async (ctx) => {
    if (!ctx.chatApi || !ctx.groupId) return "No API access."
    const targetName = ctx.args[0]
    if (!targetName) return "Usage: !kick <name>"
    try {
      const members = await ctx.chatApi.apiListMembers(ctx.groupId)
      if (!members || !Array.isArray(members)) return "Could not retrieve member list."
      const target = members.find((m: any) =>
        (m.localDisplayName || "").toLowerCase() === targetName.toLowerCase() ||
        (m.memberProfile?.displayName || "").toLowerCase() === targetName.toLowerCase()
      )
      if (!target) return `Member "${targetName}" not found.`
      await ctx.chatApi.apiRemoveMembers(ctx.groupId, [target.groupMemberId])
      logger.info(`Member kicked: ${targetName} from group ${ctx.groupId}`)
      return `${targetName} has been removed from the group.`
    } catch (err) {
      logger.error(`Error in !kick: ${err}`)
      return "Error removing member. Am I an admin in this group?"
    }
  },
})

registerCommand({
  name: "warn",
  description: "Warn a member (3x = auto-kick)",
  groupOnly: true,
  handler: async (ctx) => {
    if (!ctx.chatApi || !ctx.groupId) return "No API access."
    const targetName = ctx.args[0]
    if (!targetName) return "Usage: !warn <name>"
    const key = `${ctx.groupId}:${targetName.toLowerCase()}`
    const count = (warnings.get(key) || 0) + 1
    warnings.set(key, count)
    if (count >= MAX_WARNINGS) {
      try {
        const members = await ctx.chatApi.apiListMembers(ctx.groupId)
        const target = (members || []).find((m: any) =>
          (m.localDisplayName || "").toLowerCase() === targetName.toLowerCase() ||
          (m.memberProfile?.displayName || "").toLowerCase() === targetName.toLowerCase()
        )
        if (target) {
          await ctx.chatApi.apiRemoveMembers(ctx.groupId, [target.groupMemberId])
          warnings.delete(key)
          logger.info(`Auto-kick: ${targetName} (${MAX_WARNINGS} warnings)`)
          return `${targetName} reached ${MAX_WARNINGS} warnings and was removed.`
        }
      } catch (err) {
        logger.error(`Error in auto-kick: ${err}`)
      }
      return `${targetName} has ${count} warnings. Auto-kick failed - am I admin?`
    }
    return `${targetName} has been warned. (${count}/${MAX_WARNINGS})`
  },
})

registerCommand({
  name: "warnings",
  description: "Check warnings for a member",
  groupOnly: true,
  handler: (ctx) => {
    const targetName = ctx.args[0]
    if (!targetName) return "Usage: !warnings <name>"
    const key = `${ctx.groupId}:${targetName.toLowerCase()}`
    const count = warnings.get(key) || 0
    return count === 0
      ? `${targetName} has no warnings.`
      : `${targetName} has ${count}/${MAX_WARNINGS} warnings.`
  },
})

registerCommand({
  name: "clearwarn",
  description: "Clear all warnings for a member",
  groupOnly: true,
  handler: (ctx) => {
    const targetName = ctx.args[0]
    if (!targetName) return "Usage: !clearwarn <name>"
    const key = `${ctx.groupId}:${targetName.toLowerCase()}`
    warnings.delete(key)
    return `Warnings cleared for ${targetName}.`
  },
})
