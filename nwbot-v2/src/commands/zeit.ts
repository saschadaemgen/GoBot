import { registerCommand } from "./index.js"

registerCommand({
  name: "time",
  description: "Show current time",
  handler: (_ctx) => {
    const now = new Date()
    const time = now.toLocaleTimeString("en-US", {
      hour: "2-digit", minute: "2-digit", second: "2-digit",
      timeZone: "Europe/Berlin",
    })
    return `Current time: ${time} (Europe/Berlin)`
  },
})
