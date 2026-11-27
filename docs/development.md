# Development

[← README](../README.md)

```bash
cp .env.example .env   # fill in values (see configuration.md)

make install-tools     # install sqlc + goose (uses the public Go proxy)
make sqlc              # regenerate sqlc code after changing queries/
make css               # rebuild the web stylesheet after editing templates
make run               # start the server (applies migrations + seeds on startup)
make build             # build binary

make migrate-up        # apply migrations manually (the app also does this on startup)
make migrate-down      # roll back one migration
make migrate-create name=add_something   # scaffold a new goose migration
```

## Local development without ngrok

Telegram only accepts publicly resolvable HTTPS redirect URIs, which is why the
login used to be untestable locally. [telegram-login-broker](../../telegram-login-broker)
solves that: it is registered in BotFather once and forwards the browser to any
loopback port afterwards. Run it in mock mode and point two variables at it:

```sh
TELEGRAM_OAUTH_BASE_URL=http://127.0.0.1:8091
TELEGRAM_EXPECTED_ISSUER=http://127.0.0.1:8091
TELEGRAM_CLIENT_ID=makeme
TELEGRAM_CLIENT_SECRET=dev-secret     # must match the broker's BROKER_CLIENTS
COOKIE_SECURE=false
```

The whole exchange can be exercised without starting the bot at all:

```sh
BROKER_URL=http://127.0.0.1:8091 BROKER_CLIENT_ID=makeme BROKER_CLIENT_SECRET=dev-secret \
  go test ./internal/web -run TestLoginAgainstALiveBroker -v
```

That test drives this package's own `OIDCClient` — not a stub — through a real
broker: authorization URL, PKCE, code exchange, id_token parsing, and a replay of
the spent code. Without `BROKER_URL` it skips, so `go test ./...` stays hermetic.

What is already proven and what still has to be checked against real Telegram is
tracked in the broker's [docs/VERIFICATION.md](../../telegram-login-broker/docs/VERIFICATION.md).

Start the binary itself with `TELEGRAM_LOCAL_MODE=true` and `WEBHOOK_URL`
unset. Without one of the two it refuses to start: an empty `WEBHOOK_URL` used
to reach `setWebhook` and **delete the webhook** of whichever bot's token was in
`TELEGRAM_TOKEN`, so a local run with the production token took production
down. Local mode never calls `setWebhook` and leaves the webhook exactly as it
was, so Telegram keeps delivering wherever it pointed before. Normally that
means this process gets no updates of its own; if the dev bot's webhook is an
old tunnel to this port, stop the tunnel or real updates will keep arriving.
`cmd/seed` and `cmd/seed-audio` do not look at any of this, not even at a
malformed `TELEGRAM_LOCAL_MODE` or `TELEGRAM_INIT_TIMEOUT`: they need only
`DATABASE_URL` and `TELEGRAM_TOKEN`.

To feed the bot an update by hand, set `TELEGRAM_WEBHOOK_SECRET` for the server
(without it `/webhook` answers 503) and send the same header Telegram would.
The value in `.env` reaches only the Go process, not your shell, so export the
same value where you run curl, and replace `<your chat id>` with your numeric
Telegram chat id. Open the chat with the dev bot and press Start first: a bot
cannot message a user who never started it, and the reply in that chat is the
result to look for. `/webhook` answers 200 either way, because the handler's
`sendMessage` error never reaches the webhook caller.

```sh
export TELEGRAM_WEBHOOK_SECRET=<the value the server was started with>
curl -X POST http://localhost:8080/webhook \
  -H "X-Telegram-Bot-Api-Secret-Token: $TELEGRAM_WEBHOOK_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"update_id":1,"message":{"message_id":1,"date":0,"chat":{"id":<your chat id>,"type":"private"},"text":"/help"}}'
```

Use a separate dev bot all the same: the scheduler and every reply send real
messages through whichever token is configured.

## Docker image

```bash
docker build --platform linux/amd64 -t makeme:local .
```

The builder stage runs on the host's own platform and cross-compiles to the
target, so this works on Apple Silicon too. Running the Go toolchain itself under
qemu crashes in `go mod download`. `tailwind.css` is not rebuilt inside the image:
`//go:embed` takes the committed file, so run `make css` first if templates
changed. `.dockerignore` keeps `.env` out of the build context. The context is
copied wholesale, and without that rule real tokens would end up in an image
layer.

`docker compose up` builds the same image and runs it next to PostgreSQL. Use a
dev bot's token in `.env`, and either `TELEGRAM_LOCAL_MODE=true` or a public
`WEBHOOK_URL` with `TELEGRAM_WEBHOOK_SECRET`; see "Local development without
ngrok" above.

## Seed data

Three JSON files are seeded automatically every time the application starts — no separate command needed:

| File | Contents | Count |
|---|---|---|
| `data/words.json` | vocabulary (english/russian/level/example/transcription) | 1100 |
| `data/grammar.json` | grammar questions (level/category/subcategory/prompt/options/answer) | 768 |
| `data/grammar_categories.json` | per-category theory shown in `/learn` | 113 |

To add content, edit the relevant JSON file and restart the app; upserts are idempotent. A missing or malformed file is logged as a warning and skipped — startup still succeeds.

`cmd/seed` is a standalone CLI for loading one of these files into the DB without starting the server (`go run ./cmd/seed data/words.json`). It is redundant with the startup seeder and only useful for one-off imports against a remote database.

IPA transcription and audio pronunciation are fetched on demand from the [Free Dictionary API](https://dictionaryapi.dev/) (no key required) the first time a user taps **🔊 Послушать** on a vocabulary card, then cached in the DB. The API's mp3 links currently return 404; the web UI then speaks the word with the browser's speech synthesis instead (see [web-ui.md](web-ui.md#listening)). The bot has no such fallback.

## Testing

Integration tests in `internal/telegram/` use TestContainers (PostgreSQL + WireMock 3.13.0). Docker must be running for those. Every other package has plain unit tests and needs no Docker.

```bash
go test ./internal/telegram/ -v -count=1   # integration suite (needs Docker)
go test -race ./...                        # everything — this is what CI runs
```

CI (`.github/workflows/ci.yaml`) has three jobs:

- **test** — on every push and PR to `master`: `make css-check`, then `go test -v -race ./...` with the Go version from `go.mod`. The runner has Docker, so the testcontainers suites run there too.
- **docker-build** — on PRs only: builds the image for `linux/amd64` without pushing, so a broken Dockerfile shows up before the merge.
- **publish** — on push to `master` or a manual run on `master`, after `test`: pushes `ghcr.io/proshik/makeme:latest` and `:<sha>` with the workflow's `GITHUB_TOKEN` (`packages: write`).

The workflow can also be started by hand (`workflow_dispatch`: **Actions → CI → Run workflow**, or `gh workflow run ci.yaml --ref master`). On `master` a manual run tests and publishes, just like a push; on any other branch it only runs `test`.

A newer push to a PR cancels the older run. On `master` a running publish is never interrupted and runs go one at a time, so `latest` is never overwritten by an older commit. GitHub keeps only the newest *pending* run, though, so in a burst of pushes an intermediate commit may get no image.
