# makeMe

Gamified English learning platform via Telegram bot. Users join groups and compete daily by completing English tasks adapted to their level.

## Tech Stack

Go, PostgreSQL (pgx/sqlc/goose), chi, [go-telegram/bot](https://github.com/go-telegram/bot)

## Bot Commands

| Command | Description |
|---|---|
| `/start` | Register and see available commands. Returning users see their current level. |
| `/newgroup Название` | Create a new group. Returns an invite code to share with friends. |
| `/join КОД` | Join a group by invite code. |
| `/mygroups` | List all groups you belong to. |

## Typical Flow

```
User A                              User B
  │                                   │
  ├─ /start                           ├─ /start
  │  "Привет, Alice! ..."             │  "Привет, Bob! ..."
  │                                   │
  ├─ /newgroup English Club           │
  │  "Группа «English Club» создана!" │
  │  "Инвайт-код: a1b2c3d4"          │
  │                                   │
  │  ── shares code with Bob ──►      │
  │                                   ├─ /join a1b2c3d4
  │                                   │  "Ты вступил в группу
  │                                   │   «English Club»!"
  │                                   │
  ├─ /mygroups                        ├─ /mygroups
  │  "1. English Club (код: a1b2c3d4)"│  "1. English Club (код: a1b2c3d4)"
```

## Environment Variables

Copy `.env.example` to `.env` and fill in the values:

| Variable | Example | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable` | PostgreSQL connection string |
| `TELEGRAM_TOKEN` | `123456:ABC-DEF...` | Bot token from [@BotFather](https://t.me/BotFather) |
| `WEBHOOK_URL` | `https://example.com/webhook` | Public URL where Telegram sends updates. For local development use [ngrok](https://ngrok.com): `ngrok http 8080`, then set the resulting `https://xxxx.ngrok-free.app/webhook` |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Development

```bash
cp .env.example .env   # fill in values (see table above)

make migrate-up        # run database migrations
make sqlc              # regenerate sqlc code
make run               # start the server
make build             # build binary
```

## Testing

Integration tests use TestContainers (PostgreSQL + WireMock). Docker must be running.

```bash
go test ./internal/telegram/ -v -count=1
```
