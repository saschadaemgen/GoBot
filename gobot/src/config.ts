// nwbot-v2/src/config.ts
// Central configuration for the bot

import { readFileSync } from "fs"

let avatarImage: string | undefined
try {
  avatarImage = readFileSync("./data/avatar.b64", "utf-8").trim()
} catch {
  avatarImage = undefined
}

export const BOT_CONFIG = {
  profile: {
    displayName: "GoBot",
    fullName: "Shield",
    image: avatarImage,
  },

  db: {
    filePrefix: "./data/nwbot",
    key: "",
  },

  welcomeMessage: "Hello! I'm GoBot v0.0.1-alpha. Type !help for a list of commands.",

  commandPrefix: "!",

  rateLimit: {
    maxRequests: 10,
    windowSeconds: 60,
  },

  logging: {
    level: process.env.NWBOT_DEBUG === "1" ? "debug" : "info",
  },
} as const

export type BotConfig = typeof BOT_CONFIG
