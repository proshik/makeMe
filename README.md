# makeMe

Gamified English learning platform via Telegram bot. Users join groups and compete daily by completing English tasks adapted to their level.

## Tech Stack

Go, PostgreSQL (pgx/sqlc/goose), chi, [go-telegram/bot](https://github.com/go-telegram/bot)

## Bot Commands

| Command | Description |
|---|---|
| `/start` | Register and see available commands. Returning users see their current level. |
| `/assess` | Take the level assessment test (10 questions, adaptive difficulty A1–C2). |
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
  ├─ /assess                          ├─ /assess
  │  "Вопрос 1/10: She ___ a student" │  (10 adaptive questions)
  │  [is] [are] [am] [be]             │  ...
  │  ...10 questions...               │  "Тест завершён! Уровень: B1"
  │  "Тест завершён! Уровень: B2"     │
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

## Database Schema

```
users
├── id              BIGSERIAL PK
├── telegram_id     BIGINT UNIQUE NOT NULL
├── username        TEXT
├── first_name      TEXT NOT NULL
├── level           TEXT NOT NULL  (A1/A2/B1/B2/C1/C2)
├── onboarded       BOOLEAN NOT NULL
├── created_at      TIMESTAMPTZ
└── updated_at      TIMESTAMPTZ

groups
├── id              BIGSERIAL PK
├── name            TEXT NOT NULL
├── invite_code     TEXT UNIQUE NOT NULL
├── created_by      BIGINT FK → users(id)
└── created_at      TIMESTAMPTZ

memberships
├── id              BIGSERIAL PK
├── group_id        BIGINT FK → groups(id) ON DELETE CASCADE
├── user_id         BIGINT FK → users(id) ON DELETE CASCADE
├── role            TEXT NOT NULL  (owner/member)
├── joined_at       TIMESTAMPTZ
└── UNIQUE(group_id, user_id)

assessment_sessions
├── id              BIGSERIAL PK
├── user_id         BIGINT FK → users(id)
├── status          TEXT NOT NULL  (in_progress/completed)
├── current_level   TEXT NOT NULL  (A1–C2)
├── determined_level TEXT          (A1–C2, NULL until completed)
├── created_at      TIMESTAMPTZ
└── completed_at    TIMESTAMPTZ

assessment_answers
├── id              BIGSERIAL PK
├── session_id      BIGINT FK → assessment_sessions(id) ON DELETE CASCADE
├── question_level  TEXT NOT NULL  (A1–C2)
├── question_text   TEXT NOT NULL
├── options         JSONB NOT NULL
├── correct_option  INT NOT NULL
├── user_option     INT NOT NULL
├── correct         BOOLEAN NOT NULL
└── created_at      TIMESTAMPTZ
```

Relations:
- `users` 1 ←→ N `memberships` N ←→ 1 `groups`
- `users` 1 ←→ N `assessment_sessions` 1 ←→ N `assessment_answers`
- `groups.created_by` → `users.id`

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
