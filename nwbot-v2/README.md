# NWBot v2 - SimpleX Chat Bot

SimpleX Chat Bot mit dem offiziellen Node.js SDK.
Kein CLI noetig - der Haskell-Core laeuft direkt embedded.

## Voraussetzungen

- Node.js >= 20
- Linux x86_64 (oder macOS arm64/x86_64)

## Schnellstart

```bash
# 1. Repo klonen / Dateien auf den Server kopieren
cd /opt/nwbot-v2

# 2. Dependencies installieren
npm install

# 3. Data-Verzeichnis erstellen
mkdir -p data

# 4. Bot starten
npm start

# Oder mit Debug-Logging:
NWBOT_DEBUG=1 npm start
```

Beim ersten Start:
- Erstellt der Bot automatisch sein Profil
- Erstellt eine SimpleX-Adresse (wird in der Konsole ausgegeben)
- Diese Adresse teilst du mit Nutzern oder nutzt sie um den Bot in Gruppen einzuladen

## Befehle

| Befehl   | Beschreibung                        |
|----------|-------------------------------------|
| !hilfe   | Alle verfuegbaren Befehle anzeigen  |
| !zeit    | Aktuelle Uhrzeit (Europe/Berlin)    |
| !datum   | Aktuelles Datum                     |
| !status  | Bot-Uptime, Memory, Node-Version    |
| !ping    | Alive-Check                         |

## Konfiguration

Alle Einstellungen in `src/config.ts`:
- Bot-Name und Profiltext
- Datenbank-Pfad und Verschluesselung
- Willkommensnachricht
- Command-Prefix (Standard: !)
- Rate Limiting (Standard: 10 Anfragen/Minute pro User)

## Eigene Befehle hinzufuegen

Neue Datei in `src/commands/` erstellen:

```typescript
// src/commands/mein-befehl.ts
import { registerCommand } from "./index.js"

registerCommand({
  name: "meinbefehl",
  description: "Macht etwas Cooles",
  handler: (ctx) => {
    return `Hallo ${ctx.userName}! Args: ${ctx.args.join(", ")}`
  },
  groupOnly: false,  // Optional: nur in Gruppen?
})
```

Dann in `src/index.ts` importieren:
```typescript
import "./commands/mein-befehl.js"
```

## Projektstruktur

```
nwbot-v2/
  package.json
  tsconfig.json
  src/
    index.ts              # Einstiegspunkt, bot.run()
    config.ts             # Zentrale Konfiguration
    commands/
      index.ts            # Command-Registry + Dispatcher
      hilfe.ts            # !hilfe
      zeit.ts             # !zeit
      datum.ts            # !datum
      status.ts           # !status
      ping.ts             # !ping
    utils/
      logger.ts           # Logging
      rate-limiter.ts     # Rate Limiting pro User
  data/
    nwbot_chat.db         # (wird automatisch erstellt)
    nwbot_agent.db        # (wird automatisch erstellt)
```

## Als systemd Service laufen lassen

```ini
# /etc/systemd/system/nwbot.service
[Unit]
Description=NWBot v2 SimpleX Chat Bot
After=network.target

[Service]
Type=simple
User=nwbot
WorkingDirectory=/opt/nwbot-v2
ExecStart=/usr/bin/node dist/index.js
Restart=always
RestartSec=10
Environment=NODE_ENV=production

[Install]
WantedBy=multi-user.target
```

```bash
# Build + Service aktivieren
npm run build
sudo systemctl enable nwbot
sudo systemctl start nwbot
sudo journalctl -u nwbot -f  # Logs anschauen
```

## Phase 2 (geplant)

- Webinterface / Dashboard
- Moderation (Spam-Filter, Nutzer kicken)
- Benachrichtigungen / Alerts
- Bridge zu Matrix/Telegram
