// nwbot-v2/src/utils/rate-limiter.ts
// Sliding window rate limiter per user

import { BOT_CONFIG } from "../config.js"
import { logger } from "./logger.js"

const userRequests = new Map<string, number[]>()

export function checkRateLimit(userId: string): boolean {
  const now = Date.now()
  const windowMs = BOT_CONFIG.rateLimit.windowSeconds * 1000
  const maxReq = BOT_CONFIG.rateLimit.maxRequests

  const timestamps = (userRequests.get(userId) ?? [])
    .filter((t) => now - t < windowMs)

  if (timestamps.length >= maxReq) {
    logger.warn(`Rate limit reached: user ${userId}`)
    return false
  }

  timestamps.push(now)
  userRequests.set(userId, timestamps)
  return true
}

export function cleanupRateLimits() {
  const now = Date.now()
  const windowMs = BOT_CONFIG.rateLimit.windowSeconds * 1000
  for (const [userId, timestamps] of userRequests.entries()) {
    const active = timestamps.filter((t) => now - t < windowMs)
    if (active.length === 0) userRequests.delete(userId)
    else userRequests.set(userId, active)
  }
}
