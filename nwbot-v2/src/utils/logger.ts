// nwbot-v2/src/utils/logger.ts
// Simple logger with levels and timestamps

type LogLevel = "debug" | "info" | "warn" | "error"

const LEVEL_PRIORITY: Record<LogLevel, number> = {
  debug: 0, info: 1, warn: 2, error: 3,
}

const LEVEL_PREFIX: Record<LogLevel, string> = {
  debug: "DBG", info: "INF", warn: "WRN", error: "ERR",
}

class Logger {
  private minLevel: LogLevel = "info"

  setLevel(level: LogLevel) { this.minLevel = level }

  private log(level: LogLevel, message: string, ...args: unknown[]) {
    if (LEVEL_PRIORITY[level] < LEVEL_PRIORITY[this.minLevel]) return
    const timestamp = new Date().toISOString().replace("T", " ").slice(0, 19)
    const prefix = LEVEL_PREFIX[level]
    const line = `${timestamp} | ${prefix} | ${message}`
    if (level === "error") console.error(line, ...args)
    else if (level === "warn") console.warn(line, ...args)
    else console.log(line, ...args)
  }

  debug(msg: string, ...args: unknown[]) { this.log("debug", msg, ...args) }
  info(msg: string, ...args: unknown[]) { this.log("info", msg, ...args) }
  warn(msg: string, ...args: unknown[]) { this.log("warn", msg, ...args) }
  error(msg: string, ...args: unknown[]) { this.log("error", msg, ...args) }
}

export const logger = new Logger()
