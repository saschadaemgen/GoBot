import { registerCommand } from "./index.js"

const startTime = Date.now()

function formatUptime(ms: number): string {
  const s = Math.floor(ms / 1000)
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const parts: string[] = []
  if (d > 0) parts.push(`${d}d`)
  if (h > 0) parts.push(`${h}h`)
  if (m > 0) parts.push(`${m}m`)
  parts.push(`${s % 60}s`)
  return parts.join(" ")
}

registerCommand({
  name: "status",
  description: "Show bot status and uptime",
  handler: (_ctx) => {
    const uptime = formatUptime(Date.now() - startTime)
    const memMB = Math.round(process.memoryUsage().heapUsed / 1024 / 1024)
    return [
      "GoBot v0.0.1-alpha Status:",
      `  Uptime:   ${uptime}`,
      `  Memory:   ${memMB} MB`,
      `  Node:     ${process.version}`,
      `  Platform: ${process.platform}/${process.arch}`,
    ].join("\n")
  },
})
