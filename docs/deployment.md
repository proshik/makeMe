# Deployment on krill

[← README](../README.md)

Production runs on krill since 2026-11-20, as a single replica pulled from GHCR;
the web UI is mounted, the bot works on the webhook. Confirmed by observation
there: signing in with real Telegram returns an `id_token` that carries the `id`
claim, a user who already existed in the bot lands in their own profile with
their level and progress, the browser keeps the `Secure` session cookie, `/ws`
held through krill's proxy for 6.5 minutes with pings answered, and
`POST /admin/seed-audio` filled `audio_url` for every level. What is still
unproven is listed under "Веб-интерфейс: что не проверено" in `docs/plan.md`.

## Where the image comes from

krill deploys ready-made images rather than building from source. CI publishes
one on every push to `master`, after the tests pass:

```
ghcr.io/proshik/makeme:<commit-sha>   # deploy this one — it pins the exact build
ghcr.io/proshik/makeme:latest
```

The name is lower-case on purpose: GHCR rejects upper case, so it is not the
repository's `makeMe`. The image is `linux/amd64` only, listens on `8080`
(`EXPOSE 8080`, matching the `PORT` default), runs as uid 10001, and carries
`migrations/` and `data/` under `/app`. Migrations and the JSON seed run on every
start, so a fresh database needs nothing beyond the variables below. The package
is public, so krill pulls it anonymously: the app's registry is "Public (no
auth)", no credential needed. Deploy by full commit sha, never `latest`.

The app's healthcheck is `GET /health`, which answers 200 without touching the
database. krill rolls out `start-first`: the new task has to start and turn
healthy before the old one stops, so a build that fails on startup (a missing
variable, `getMe` failing) leaves the previous version running. That is what
makes a startup error the safe way to reject a bad configuration.

## Environment on krill

Values live in krill's secret store, never in the repository.

| Variable | Why it is required |
|---|---|
| `DATABASE_URL` | Startup fails without it. |
| `TELEGRAM_TOKEN` | Startup fails without it. The **production** bot. |
| `WEBHOOK_URL` | `https://<host>/webhook`. Startup fails with it empty (unless `TELEGRAM_LOCAL_MODE=true`, which production must not set). |
| `TELEGRAM_WEBHOOK_SECRET` | `openssl rand -hex 32`. Startup fails without it while `WEBHOOK_URL` is set. Add it **before** deploying a build that requires it: the old build ignores the variable, the new one refuses to start without it. |
| `APP_BASE_URL` | Public origin; the redirect URI is derived from it. |
| `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET` | OAuth pair from BotFather. Without them the web UI is not mounted. |
| `JWT_SECRET` | Session key, ≥32 bytes base64. Without it the web UI is not mounted. |
| `ADMIN_TOKEN` | Needed for `POST /admin/seed-audio`; without it every `/admin/*` returns 503. |

Leave `TELEGRAM_OAUTH_BASE_URL`, `TELEGRAM_EXPECTED_ISSUER`, `COOKIE_SECURE` and
`TELEGRAM_LOCAL_MODE` at their defaults in production: the first two point at
Telegram itself, the cookie must stay `Secure`, and local mode would leave the
webhook unregistered. `TELEGRAM_INIT_TIMEOUT` (30s), `PORT`, `TIMEZONE`,
`SCHEDULER_TIME` and the rest are optional (see [configuration.md](configuration.md)).

## Turning on the webhook secret

A build that requires `TELEGRAM_WEBHOOK_SECRET` is rolled out in this order:

1. Generate the value with `openssl rand -hex 32` and add it to the app's
   environment **first**. The running build does not read it. If saving the
   variable restarts the app, the restarted old build calls `setWebhook` without
   a secret, as it always did.
2. Deploy the new build by sha. It registers the webhook with the secret only
   after everything else has started and its port is bound, right before it
   serves (`telegram webhook set` in the log). From then on Telegram sends the
   header. While both tasks run, the old one accepts every request and the new
   one checks the header, so nothing is refused. Had the variable been
   forgotten, the log names it (`TELEGRAM_WEBHOOK_SECRET is required when
   WEBHOOK_URL is set`), the new task fails before migrations and before any
   Telegram call, never turns healthy, and the old one keeps serving with the
   webhook as it was.
3. Check after the deploy: `getWebhookInfo` shows the same URL, a pending count
   of 0 and no `last_error_message` (a 401 there means Telegram holds a
   different secret than the app); `/start` in the bot answers; a `POST /webhook`
   without the header gets 401.

If a deploy fails after `telegram webhook set` appeared in its log, or
`setWebhook` itself returned an error, check `getWebhookInfo` before anything
else: Telegram may have dropped or changed the registration while the old task
kept serving. Restarting the serving task re-registers its own URL and secret.

**Changing the secret later** (a new value, same URL) refuses updates for a
short while: once the new task registers the new value, the old task still
expects the old one and answers 401 to whatever reaches it until it stops.
Telegram retries a failed delivery, but only a limited number of times, so a
short cutover delays updates while a long one, or a failed new task left
behind by the old one, can lose them. A pending count back at 0 does not tell
the two apart, since dropped updates leave the queue too. After the cutover,
compare `last_error_date` in `getWebhookInfo` with the time of the deploy and
confirm delivery with a fresh `/start`. If 401s keep coming, restart the task
that is serving so it registers its own secret again.

**Rolling back to a build without the check** gives up the authentication, and
the cutover is the same short window in reverse: the old build calls
`setWebhook` without `secret_token` on startup (before its own port is open),
and until the hardened task stops, Telegram's requests without the header reach
it and get 401. Telegram retries them a limited number of times, so the same
caveat applies: a short cutover delays updates, a long one can lose them. If
the rolled-back task then fails to start, the hardened one keeps serving
against a registration with no secret and refuses everything; redeploy or
restart the hardened build so it registers its secret again, then confirm with
`getWebhookInfo` and a fresh `/start`.

## Steps for a new environment

1. Register an OAuth application for the bot and obtain `TELEGRAM_CLIENT_ID` / `TELEGRAM_CLIENT_SECRET`. These are **not** the same as `TELEGRAM_TOKEN`.
2. `openssl rand -base64 32` → `JWT_SECRET`. Startup fails if it is shorter than 32 bytes decoded, so a bad value surfaces immediately rather than on first login. `openssl rand -hex 32` → `TELEGRAM_WEBHOOK_SECRET` (hex, because base64 falls outside Telegram's character set).
3. Set `APP_BASE_URL` to the public origin. The redirect URI is derived as `$APP_BASE_URL/auth/telegram/callback` and must match what was registered exactly.
4. Deploy `ghcr.io/proshik/makeme:<sha>` with the environment above. In the logs expect `migrations applied`, three `seed:` lines, `web UI enabled` and then `telegram webhook set` right before `starting server`; then confirm in this order, since each step depends on the previous one working:
   - `GET /login` renders and the button leads to Telegram.
   - Signing in lands back on `/` as the right user. A user who already exists in the bot must keep their level and progress.
   - A WebSocket client holding the `auth_token` cookie keeps `wss://<host>/ws` open longer than the platform's idle timeout. The server pings every 25s; a shorter proxy timeout will cut the socket. On krill it held 6.5 minutes.
   - Answer a task in the bot with the browser open on `/leaderboard`: the standings should move without a reload.
5. **After the first deploy, run `POST /admin/seed-audio`** level by level (see [Production workflow](admin.md#production-workflow)). `audio_url` is not in `data/words.json` and the startup seeder never fills it, so on a fresh database it is empty for every word. Until then the bot's "Послушать" fetches each word on its first tap, and the web UI, which never calls the API, falls back to the browser's speech synthesis.
