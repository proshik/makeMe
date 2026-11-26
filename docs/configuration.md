# Configuration

[← README](../README.md)

Copy `.env.example` to `.env` and fill in the values:

| Variable | Example | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://makeme:makeme@localhost:5432/makeme?sslmode=disable` | PostgreSQL connection string |
| `TELEGRAM_TOKEN` | `123456:ABC-DEF...` | Bot token from [@BotFather](https://t.me/BotFather) |
| `WEBHOOK_URL` | `https://example.com/webhook` | Public URL where Telegram sends updates. **Required unless `TELEGRAM_LOCAL_MODE=true`**: startup refuses an empty value, because setting an empty webhook deletes the bot's webhook. It must be an absolute `https://` URL with a host, a port Telegram delivers to (443 when omitted, or 80, 88, 8443; the public port, not `PORT`) and the path `/webhook`; anything else is refused at startup too, since Telegram can drop the current webhook before rejecting a URL it does not accept. The value is not repeated in the error, as it may carry credentials. For local development with real updates use [ngrok](https://ngrok.com): `ngrok http 8080`, then set the resulting `https://xxxx.ngrok-free.app/webhook` |
| `TELEGRAM_LOCAL_MODE` | `false` | `true` starts the bot without touching its webhook: no `setWebhook` call, so Telegram keeps delivering wherever the webhook already points. Usually that is nowhere or another environment, and this process gets only what is posted to `/webhook` by hand; if the existing webhook is a tunnel to this very port and the secrets match, real updates arrive. Outgoing messages still work. Mutually exclusive with `WEBHOOK_URL` (startup fails if both are set). Must parse as a boolean; a typo is an error, not `false`. |
| `TELEGRAM_WEBHOOK_SECRET` | `(empty)` | Secret Telegram sends back in `X-Telegram-Bot-Api-Secret-Token` on every update; `/webhook` answers 401 without it. **Required whenever `WEBHOOK_URL` is set**: startup fails otherwise. 1–256 characters of `A-Za-z0-9_-` (Telegram's own limit, checked at startup): `openssl rand -hex 32`. Base64 does not fit, since `+`, `/` and `=` are outside the set. With it empty `/webhook` answers 503. |
| `TELEGRAM_INIT_TIMEOUT` | `30s` | How long startup waits for `getMe`, the call `go-telegram/bot` makes before the bot exists (its own default is 5s). A Go duration (`30s`, `1m`), at most `60s`: the library's HTTP client cuts every request at one minute, so a longer wait would never happen. A bad, non-positive or larger value is a startup error, not a silent default. |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `WORDS_FILE` | `data/words.json` | Word list seeded on every startup |
| `GRAMMAR_QUESTIONS_FILE` | `data/grammar.json` | Grammar questions seeded on every startup |
| `GRAMMAR_CATEGORIES_FILE` | `data/grammar_categories.json` | Grammar category explanations (`/learn` theory) seeded on every startup |
| `SCHEDULER_TIME` | `21:00` | Time to send daily summaries (HH:MM, 24h) |
| `TIMEZONE` | `Europe/Moscow` | Timezone for the scheduler |
| `MAX_ROUNDS_PER_DAY` | `5` | Max task rounds a user can run per day (one round = 10 tasks) |
| `ADMIN_TOKEN` | `(empty)` | Bearer token for `/admin/*` HTTP endpoints. Leave empty to disable them (they return 503). Generate with `openssl rand -hex 32`. |
| `APP_BASE_URL` | `https://makeme.example.com` | Public origin of the web UI. `redirect_uri` is derived as `$APP_BASE_URL/auth/telegram/callback` — register that exact URL with Telegram. |
| `TELEGRAM_CLIENT_ID` | `(empty)` | OAuth client id. Separate from `TELEGRAM_TOKEN`. |
| `TELEGRAM_CLIENT_SECRET` | `(empty)` | OAuth client secret. |
| `JWT_SECRET` | `(empty)` | Base64 key for session JWTs, ≥32 bytes decoded. `openssl rand -base64 32`. Startup fails if it is shorter or not base64. |
| `JWT_TTL_HOURS` | `24` | Session lifetime; the `auth_token` cookie max-age is derived from it. |
| `COOKIE_SECURE` | `true` | Set `false` for local http development. Chrome and Firefox accept a `Secure` cookie from `http://localhost`; Safari drops it silently, and no browser accepts it from `http://192.168.x.x`, so testing from a phone needs this too. |
| `TELEGRAM_OAUTH_BASE_URL` | `https://oauth.telegram.org` | OIDC provider base. Point it at a login broker for local development. |
| `TELEGRAM_EXPECTED_ISSUER` | `https://oauth.telegram.org` | The exact `iss` the id_token must carry, compared byte for byte. Changed together with the base URL and never derived from it: pointing the flow somewhere else has to be deliberate. |

The web UI is optional: with `APP_BASE_URL`, `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET` or `JWT_SECRET` unset it is not mounted and only the bot runs. A warning is logged at startup saying which variables are missing.

Local development (login broker, local mode, feeding `/webhook` by hand) is in
[development.md](development.md#local-development-without-ngrok); the production
environment is in [deployment.md](deployment.md#environment-on-krill).
