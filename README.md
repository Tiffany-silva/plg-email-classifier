# Email Classification Service

A standalone Go HTTP service that classifies a signup email into a category and
a 0–100 lead-quality **rating**, for PLG customer-success triage. It uses a
cheap-to-expensive cascade: deterministic rules and a domain cache resolve most
requests with no model call; only unknown-but-reachable domains are sent to the
Anthropic API (Claude Haiku 4.5, structured outputs).

This repository is self-contained — its own Go module with **no external
dependencies** — and follows the CSM portal backend's structure and conventions
(the `SecurityHeaders → CORS → CorrelationID → Auth → Logger` middleware chain,
`internal/apierror`, `internal/handler` with `response.go`, the upstream-client
pattern under `internal/anthropic`, and the `cmd/server/main.go` bootstrap
style). It is not part of, and does not depend on, the CSM portal repo.

## Quick start

```bash
cp .env.example .env      # set ANTHROPIC_API_KEY, or LLM_ENABLED=false to skip it
go run ./cmd/server
```

The server auto-loads `.env` from the working directory (ignored if absent) and
starts on `http://localhost:8080`.

## Commands

```bash
make test    # go vet + race-detector tests
make build   # runs tests then compiles ./cmd/server
make run     # go run ./cmd/server
```

## Layout

```
cmd/server/           entrypoint, routing, middleware chain, graceful shutdown
internal/apierror/    typed upstream error
internal/middleware/  security headers, CORS, correlation ID + slog, auth, logger
internal/anthropic/   upstream client for the Anthropic Messages API
internal/emailclassifier/  cascade, rules, lists, scoring, cache, refresher
internal/handler/     HTTP handlers (health, classify-email) + response helpers
```

## API

`POST /classify-email`

```bash
curl -s http://localhost:8080/classify-email \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@acme.com"}'
```

Add `-H 'X-API-Key: <key>'` when `CALLER_API_KEYS` is set. Categories:
`corporate`, `personal`, `provider_testing`, `disposable`, `unknown`, `invalid`.
See `openapi.yaml` for the full contract.

## Auth

Identity comes from an `X-API-Key` allow-list (`CALLER_API_KEYS`); unset disables
auth for local dev. When deploying behind a gateway that already validates a JWT,
replace `middleware.Auth` with a JWT variant — keep the same
`middleware.UserInfoFromContext` contract and the handlers need no changes.

## Behaviour notes

- **Graceful degradation:** if the model errors or times out, the request never
  fails — it returns `unknown` with low confidence, never a confident guess.
  `unknown`/`invalid` verdicts are never cached (they are transient).
- **Disposable list:** loaded from `DISPOSABLE_LIST_PATH`, then auto-refreshed
  in the background from `DISPOSABLE_LIST_URL` every `DISPOSABLE_REFRESH_INTERVAL`
  (default 2h), swapped atomically, and written back to the file. A bad/empty
  fetch keeps the previous list. Set the interval to `0` to disable polling.
- **Keyless testing:** `LLM_ENABLED=false` runs the deterministic cascade only.
