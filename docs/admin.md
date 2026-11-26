# Admin endpoints

[← README](../README.md)

Protected by `Authorization: Bearer $ADMIN_TOKEN`. Disabled when `ADMIN_TOKEN` is empty (responds 503).

## `POST /admin/seed-audio`

Bulk-fetches transcription + audio_url from the Free Dictionary API for words at the given CEFR levels and persists them to the `words` table. Idempotent: words that already have `audio_url` are skipped.

Body (JSON):
```json
{ "levels": ["A1"], "delay_ms": 1500 }
```

**Pass `delay_ms: 1500` explicitly.** It is the pause between calls to `api.dictionaryapi.dev`, and on production 500 ms got throttled: 429s came in waves, about 20 words through and the next 20 refused, ~40% of A1 failed. At 1500 ms whole levels went through without a single 429. The server's own default when `delay_ms` is omitted is still 500 ms, and `cmd/seed-audio` defaults to 150 ms.

Response: streamed `application/x-ndjson`, one line per event (start of level / per-word result / final summary). Use `curl -N` so output isn't buffered.

A run lasts minutes, far longer than the server's 10s `WriteTimeout`, which is a deadline on the whole response rather than on each line. For this response only, the handler replaces it with a 30s deadline on each line, re-armed right before every write; every other endpoint keeps the global one. The gap between lines does not count, so a slow dictionary lookup or a large `delay_ms` does not cut the stream. Without that the stream was cut after ~10s with a few lines delivered and no `done`. The per-line bound is what still releases a run whose client keeps the connection open but stops reading: the write fails, net/http cancels the request context, the seeder stops and the handler returns. The handler's own `Flush` check does not see that failure behind chi, whose wrapper implements plain `http.Flusher`, so the cancelled context is what marks the run as cut. `TestSeedAudio_StreamOutlivesServerTimeouts`, `TestSeedAudio_StalledClientReleasesRun` and `TestSeedAudio_ClientDisconnectCancelsRun` run the handler behind a real `http.Server` with sub-second timeouts and the same middleware as production — the deadlines reach the connection only through the middleware's `Unwrap`, so a wrapper without it would turn every run into a 500; flush errors, by contrast, stop at chi's wrapper. On production, runs of 4.5 and 6 minutes reached `done` through krill's proxy intact. A single 30-minute run over five levels did not: the server wrote everything and logged `completed`, but the client got the stream without its last ~2 KB and no `done`, and exited after four minutes of silence. Where the tail was lost was not established. The data was unaffected, because writes happen on the server; the outcome was checked in the database. Hence the advice below to send one or two levels per request.

```bash
curl -N -X POST https://your-host.example.com/admin/seed-audio \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"levels":["A1"],"delay_ms":1500}'
```

For local seeding without HTTP, the same logic ships as `cmd/seed-audio`:
```bash
go run ./cmd/seed-audio -levels=A1 -delay-ms=1500
```

### Production workflow

`audio_url` is **not** part of `data/words.json` and is **not** copied across environments by the on-startup seeder (`UpsertWord` only touches english/russian/level/example/transcription). After deploying to production, populate audio with:

1. Generate and set a token (do this once, store in your secret manager):
   ```bash
   openssl rand -hex 32   # → 64 hex chars
   ```
   Set the value as `ADMIN_TOKEN` in the production environment and redeploy so the service picks it up.

2. From any machine with HTTPS access to prod, run **one or two levels per request**, with `delay_ms` 1500:
   ```bash
   curl -N -X POST https://your-prod.example.com/admin/seed-audio \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"levels":["A1"],"delay_ms":1500}'
   ```
   Stream of ndjson events ends with a `{"event":"done", ...}` summary line. Each word without audio costs one API call plus `delay_ms`, so budget about 1.5 s per word still missing audio: on production A1 and A2 hold 520–560 words each, ~13–14 minutes per level from scratch, while B1 to C2 hold ~150 each, so two of them fit in ~8 minutes. Words that already have audio cost nothing. Long streams are where the tail went missing on production (see above); short ones arrived whole.

3. **If 429 appears** (`"status":"failed","error":"unexpected status: 429"`), Free Dictionary throttled you: `delay_ms` is too low. Re-run the same levels with 1500; the run is idempotent, so already-populated rows are skipped. Words that came back `not_found` have no `audio_url` either, so every re-run asks the API about them again and spends `delay_ms` on each.

4. **Adding more levels** later: same call, change `levels`, still one or two at a time.

### Limits to expect

* Free Dictionary returns no audio at all for ~3–5% of common words (e.g. `look`, `room`, `sea`); they end up as `"status":"not_found"` and stay with empty `audio_url`. The user-facing `🔊 Послушать` button will reply "Произношение не найдено" for these — acceptable for v1.
* Disconnect (closing curl, network drop) is handled: the server cancels the run via `ctx.Done()` so no quota is wasted on a dropped client. The server log reports what the server saw. `admin seed-audio aborted before done`, logged once with the cause, means its own write of a line failed or the request was cancelled: the client left, stopped reading, or the connection was cut before `done` was written. `completed` means the server wrote `done` without an error, which is not proof that the client got it: a proxy can accept the last bytes and lose them, as happened once on production. When a client saw no `done`, look for either line, and if it says `completed`, check `audio_url` in the database before re-running. A re-run is safe either way, since it skips words that already have audio.
