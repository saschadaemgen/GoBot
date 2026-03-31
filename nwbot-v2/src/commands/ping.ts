import { registerCommand } from "./index.js"

registerCommand({
  name: "ping",
  description: "Check if bot is online",
  handler: (_ctx) => "pong",
})
