import { BOT_CONFIG } from "../config.js"
import { registerCommand, getCommands } from "./index.js"

registerCommand({
  name: "help",
  description: "Show available commands",
  handler: (_ctx) => {
    const prefix = BOT_CONFIG.commandPrefix
    const cmds = getCommands()
    const lines = cmds.map((c) => `  ${prefix}${c.name} - ${c.description}`)
    return ["GoBot v0.0.1-alpha - Commands:", "", ...lines].join("\n")
  },
})
