import { registerCommand } from "./index.js"

registerCommand({
  name: "date",
  description: "Show current date",
  handler: (_ctx) => {
    const now = new Date()
    const date = now.toLocaleDateString("en-US", {
      weekday: "long", year: "numeric", month: "long", day: "numeric",
      timeZone: "Europe/Berlin",
    })
    return `Date: ${date}`
  },
})
