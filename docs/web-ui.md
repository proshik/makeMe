# Web UI

[← README](../README.md)

| Route | Description |
|---|---|
| `GET /login` | Sign-in page. Redirects to `/` if already signed in. |
| `GET /auth/telegram` | Starts OAuth: mints `state` + PKCE verifier into 5-minute HttpOnly cookies, redirects to Telegram. |
| `GET /auth/telegram/callback` | Verifies `state`, exchanges the code for an `id_token`, resolves the Telegram identity via `user.Service.Register` (idempotent by `telegram_id`, so bot users keep their row), sets the `auth_token` cookie. |
| `GET /logout` | Clears the session cookie. |
| `GET /` | Home. Requires auth. |
| `GET /tasks` | Today's round. Generates or resumes it, auto-creating a solo group if the user has none — the same rule the bot follows. |
| `GET /tasks/next` | Next slot as an htmx fragment. Deliberately reads the *current* set instead of generating: `GenerateDaily` opens the next round once the current one is answered, so generating here would roll the user into a new round instead of showing the summary. |
| `POST /tasks/{id}/answer` | Records a choice, a typed answer or a skip via `task.Service`, scores it, and returns the feedback fragment. |
| `GET /assess` | Starts or resumes the 30-question level test. The current level is deliberately not shown during the run — it moves adaptively after every answer, and showing it would turn the test into a game against the system. |
| `POST /assess/answer` | Records an option or a skip, returns the next question or the result — level, score, and a per-question breakdown so the verdict is justified. |
| `GET /words` | Opens or resumes the ten-word active-recall session, with per-mode tallies and live per-CEFR-level progress. |
| `GET /words/next` | Next drill as a fragment. Completing the last one closes the session, so the following visit picks a fresh ten. |
| `POST /words/{id}/answer` | Records a choice, a typed answer or a skip. Multiple-choice correctness is decided server-side from the submitted text — the correct index is never sent to the browser. |
| `POST /words/{id}/know` | Retires a word the user already knows across all four modes and drops it into spaced repetition. |
| `POST /words/{id}/defer` | Drops a word from the rest of the session, keeping its progress. Offered only past `words.DeferThreshold` mistakes on that word. |
| `GET /learn` | Grammar topics at the user's level and the one above it. |
| `GET /learn/topic?name=…` | Theory plus a fresh random seed, so re-entering a topic reshuffles the order. |
| `GET /learn/practice?name=…&seed=…&i=…` | One question of a run. The run keeps no server-side state: seed plus index reproduce the same sample, so a refresh does not lose the user's place. |
| `POST /learn/answer` | Checks the answer and returns the feedback fragment. The rule is shown only when the answer was wrong — after a correct one it is noise. |
| `GET /leaderboard` | Group standings, day or month. `?group=` is honoured only for groups the user belongs to — any other id falls back to their own, so an id cannot expose another group's standings. |
| `GET /profile` | Level, monthly score, streak, vocabulary split by how each word got there, per-CEFR-level progress, group list. |
| `GET /groups`, `POST /groups/create`, `POST /groups/join` | Create a group (returns an invite code) or join by code. A wrong code and a missing group give the same message, so codes cannot be probed. |
| `GET /static/*` | Self-hosted assets, served from the binary via `embed.FS`. |

Topic names travel as query values rather than path segments: categories like `can/can't` carry slashes, which a path segment would mangle.

`/learn` and `/words` write no score rows — not contributing to the leaderboard is its one behavioural difference from `/tasks`, and a test guards it.

Asset URLs carry a `?v=` token derived from a hash of every embedded file, so `Cache-Control: immutable` is safe — a new build produces new URLs. Without it a deploy would leave browsers on the previous stylesheet for a year.

Session cookies are `HttpOnly`, `Secure`, `SameSite=Lax` — Lax is the CSRF defence for cookie auth, which is why the POST handlers carry no separate CSRF token. The `id_token` signature is deliberately not verified: it arrives from Telegram's token endpoint over TLS server-to-server, and there is no public JWKS to check against; `exp`, `aud` and `iss` are validated as defence in depth.

## Live updates

`GET /ws` is a WebSocket carrying event names, not rendered HTML. A tab may be on any page, so the server cannot know what to swap; it says what changed and the client refetches through the ordinary handlers, keeping one rendering path for both the first load and the update.

Two events exist. `progress` goes to the user's own sessions when they answer anywhere. `leaderboard` goes to everyone in the group, and to the user themselves, but only when the answer actually scored — a wrong answer cannot move the standings.

What each page does with an event is a deliberate split. The leaderboard refetches itself (`hx-trigger="makeme:leaderboard from:body"`): it is a passive page with nothing to lose. Tasks and words only reveal a notice offering to reload — pulling the slot out from under someone who is typing an answer is worse than showing them something slightly stale.

Four things this needed that are easy to miss:

- `http.Server`'s `WriteTimeout` is the usual suspect for a socket that dies on a timer, and this server sets one — but it does not apply. `net/http` clears both deadlines when a connection is hijacked, which `websocket.Accept` does. `TestSocketOutlivesWriteTimeout` pins that down against a deliberately short timeout, so the assumption is rechecked if the value or the library ever changes.
- The hub cannot tell sessions apart, so the tab that answered also receives `progress`. Threading a socket identity through a domain event would drag transport into the domain; the client suppresses the notice for three seconds after its own request instead.
- The hub is required. `realtime.js` is on every page and always opens `/ws`, so a web UI without a hub is broken rather than degraded: the first production deploy shipped with the hub created but never passed into `web.Deps`, `/ws` answered 404 and every tab kept reconnecting, with nothing in the logs. `web.New` now returns `ErrNoHub` instead, and the binary refuses to start (`web: Deps.Hub is nil; the web UI needs the realtime hub`), so the same slip fails the deploy. With the web UI disabled no hub is needed.
- The hub is in-process, so events reach only sessions on this instance. The domain talks to `task.Notifier`, not to the hub, so a Postgres `LISTEN/NOTIFY` implementation can replace it without the domain noticing.

## Navigation

The header carries two levels, because it holds two different kinds of thing. On the left are places to work — Задания, Слова, Грамматика, Рейтинг — with the current one marked by an amber underline and `aria-current="page"`. On the right is the user: a chip with their name and level opening a menu with Профиль, Группы, appearance and Выйти.

That menu is a native `<details>`, not a scripted dropdown: `script-src 'self'` forbids inline handlers, and a disclosure element is keyboard-accessible with no JavaScript at all. `app.js` adds only what `<details>` lacks — closing on an outside click and on Escape.

Below 720px the account name is hidden (the level chip stays, since it carries information) and the sections scroll horizontally rather than wrapping. The breakpoint is measured, not guessed: at 606px the four sections overflowed by 20px while the name still occupied the chip.

## Assets are self-hosted, and Tailwind is pre-built

Everything the pages load comes from our own origin (`internal/web/static/`, embedded into the binary): the built Tailwind stylesheet, htmx 2.0.4, and the four fonts of the «Табло» system — Oswald, IBM Plex Sans, IBM Plex Mono, and Charis SIL for IPA transcriptions.

Two separate decisions here, for two separate reasons.

**Nothing is loaded from a CDN.** A CDN means depending on someone else's uptime and — the deciding factor — being unable to close the CSP. With everything local the app ships `default-src 'self'; script-src 'self'`, with no `'unsafe-inline'` for scripts. That in turn constrains the templates: no inline `<script>`, no `on*=""` handlers, no htmx `hx-on` / `js:` expressions — behaviour lives in `static/js/app.js` and hangs off `data-action` attributes. Four tests guard all of this: the CSP directives, the absence of `unsafe-inline` in `script-src`, the absence of inline scripts and handlers in rendered pages, and any page referencing an external host.

`style-src` does keep `'unsafe-inline'`: templates carry inline `style=""` for design-token colours, and style injection is far less dangerous than script execution — an accepted trade-off rather than a nonce pipeline.

**Tailwind is compiled ahead of time, not in the browser.** `cdn.tailwindcss.com` is the Play CDN, which Tailwind documents as development-only: it ships the compiler and generates CSS on every page load. Instead, `make css` runs the standalone Tailwind CLI (a single binary — no Node, no `npm install`) over `internal/web/templates/**` and writes `internal/web/static/tailwind.css`. That output is **committed**, because `//go:embed` needs the file present at compile time — which also means `docker build` stays a plain Go build with no asset step.

```bash
make css         # rebuild after editing templates
make css-check   # fails if the committed CSS is stale (runs in CI)
```

`tailwind.config.js` zeroes the `borderRadius` and `boxShadow` scales outright rather than via `extend`, so `rounded-lg` and `shadow-md` do not exist at all — the design system forbids radii and shadows, and a test asserts they never appear in the built CSS.
