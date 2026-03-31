# GoBot

A SimpleX Chat group bot built with the official `simplex-chat` Node.js SDK. No CLI dependency - the Haskell core runs embedded via native FFI.

## Requirements

- Node.js >= 20
- Linux x86_64 or macOS arm64/x86_64

## Quick Start

```bash
cd gobot
mkdir -p data
npm install
npm run build
npm start
```

On first launch the bot creates its profile, generates a SimpleX address (printed to stdout), and starts listening for messages. Share the address to let users connect.

## Commands

| Command | Description |
|---------|-------------|
| !help | Show available commands |
| !time | Current time (Europe/Berlin) |
| !date | Current date |
| !status | Bot uptime, memory, node version |
| !ping | Check if bot is online |
| !members | List group members |
| !kick \<name\> | Remove a member from the group |
| !warn \<name\> | Warn a member (3x = auto-kick) |
| !warnings \<name\> | Check warnings for a member |
| !clearwarn \<name\> | Clear warnings for a member |

## Configuration

All settings in `src/config.ts`: bot profile, database path, welcome message, command prefix, rate limiting.

## Adding Commands

Create a file in `src/commands/`:

```typescript
import { registerCommand } from "./index.js"

registerCommand({
  name: "mycommand",
  description: "Does something cool",
  handler: (ctx) => `Hello ${ctx.userName}!`,
  groupOnly: false,
})
```

Import it in `src/index.ts`:

```typescript
import "./commands/mycommand.js"
```

## Running as a Service

```ini
# /etc/systemd/system/gobot.service
[Unit]
Description=GoBot SimpleX Chat Bot
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gobot
ExecStart=/usr/bin/node dist/index.js
Restart=always
RestartSec=10
Environment=NODE_ENV=production

[Install]
WantedBy=multi-user.target
```

```bash
systemctl enable gobot
systemctl start gobot
journalctl -u gobot -f
```

## Project Structure

```
gobot/
  src/
    index.ts            # Entry point, bot.run()
    config.ts           # Central configuration
    commands/
      index.ts          # Command registry + dispatcher
      hilfe.ts          # !help
      zeit.ts           # !time
      datum.ts          # !date
      status.ts         # !status
      ping.ts           # !ping
      mod.ts            # !kick !warn !members !warnings !clearwarn
    utils/
      logger.ts         # Logging
      rate-limiter.ts   # Per-user rate limiting
  data/
    avatar.b64          # Bot avatar (base64 data URI)
    nwbot_chat.db       # Chat database (auto-created)
    nwbot_agent.db      # Agent database (auto-created)
```

## License

MIT
