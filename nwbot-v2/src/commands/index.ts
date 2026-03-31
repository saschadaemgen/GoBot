// nwbot-v2/src/commands/index.ts
// Command registry: register, parse, and dispatch commands

import { BOT_CONFIG } from "../config.js"
import { logger } from "../utils/logger.js"

export interface CommandContext {
  rawText: string
  args: string[]
  userId: string
  userName: string
  chatType: "group" | "direct"
  groupId?: number
  groupName?: string
  chatApi?: any
  chatItem?: any
}

export type CommandHandler = (ctx: CommandContext) => string | null | Promise<string | null>

export interface CommandDef {
  name: string
  description: string
  handler: CommandHandler
  groupOnly?: boolean
  dmOnly?: boolean
  adminOnly?: boolean
}

const commands = new Map<string, CommandDef>()

export function registerCommand(cmd: CommandDef) {
  const key = cmd.name.toLowerCase()
  if (commands.has(key)) {
    logger.warn(`Command "${key}" is being overwritten`)
  }
  commands.set(key, cmd)
  logger.debug(`Command registered: ${BOT_CONFIG.commandPrefix}${key}`)
}

export function getCommands(): CommandDef[] {
  return Array.from(commands.values())
}

export async function dispatch(ctx: CommandContext): Promise<string | null> {
  const prefix = BOT_CONFIG.commandPrefix
  const text = ctx.rawText.trim()

  if (!text.startsWith(prefix)) return null

  const withoutPrefix = text.slice(prefix.length)
  const parts = withoutPrefix.split(/\s+/)
  const cmdName = (parts[0] ?? "").toLowerCase()
  const args = parts.slice(1)

  if (!cmdName) return null

  const cmd = commands.get(cmdName)
  if (!cmd) {
    logger.debug(`Unknown command: ${prefix}${cmdName}`)
    return null
  }

  if (cmd.groupOnly && ctx.chatType !== "group") {
    return "This command only works in groups."
  }
  if (cmd.dmOnly && ctx.chatType !== "direct") {
    return "This command only works in direct messages."
  }

  try {
    const result = await cmd.handler({ ...ctx, args })
    return result
  } catch (err) {
    logger.error(`Error in command ${prefix}${cmdName}: ${err}`)
    return "Internal error while executing command."
  }
}
