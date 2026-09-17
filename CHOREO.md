# Deploying to Choreo

This service is a Choreo **Service** component built from a **Dockerfile**.

## What's already in the repo for Choreo

- `Dockerfile` — multi-stage, static binary on distroless, running as a non-root
  numeric UID (`10014`) in Choreo's required 10000-20000 range.
- `.choreo/component.yaml` — declares the REST endpoint (port 8080) and points
  at `openapi.yaml`. Network visibility is `Organization` (internal); change to
  `Project` for tighter scope or `Public` to expose it via API management.
- `openapi.yaml` — the API contract Choreo publishes for the endpoint.

## One-time: create the component

1. Push this repo to GitHub (its own repo).
2. In the Choreo Console, open your organization/project and
   **Create → Service**.
3. Authorize the Choreo GitHub app and select this repository and branch.
4. Build preset: **Dockerfile**.
   - Dockerfile path: `Dockerfile`
   - Build context: `/` (repo root)
   - Component (endpoint) directory: `/` (where `.choreo/component.yaml` lives)
5. Create. Choreo reads `.choreo/component.yaml` for the endpoint (port 8080).

## Configure (per environment): Configs & Secrets

Add these under the component's **Configs & Secrets** (as environment variables):

Secret:
- `ANTHROPIC_API_KEY` = your key

Config (env vars):
- `LLM_ENABLED` = `true`
- `CLASSIFIER_MODEL` = `claude-haiku-4-5-20251001`
- `CACHE_TTL` = `48h`
- `LLM_TIMEOUT` = `8s`
- `MX_TIMEOUT` = `3s`
- `DISPOSABLE_LIST_URL` = `https://dmails.doodadlabs.org/data/domains.txt`
- `DISPOSABLE_REFRESH_INTERVAL` = `2h`

Leave `DISPOSABLE_LIST_PATH` **unset** so the service keeps the blocklist in
memory only (the container filesystem is ephemeral). The refresher fetches the
full list on startup and every interval; no writable volume is needed. If you do
set it, use `/app/data/disposable_domains.txt` (writable by UID 10014).

`PORT` defaults to 8080 and matches `component.yaml`; no need to set it.

## Auth

With `Organization`/`Project` visibility, Choreo restricts callers to inside the
org/project, and you apply Choreo's managed authentication at the gateway. In
that setup leave `CALLER_API_KEYS` unset (the in-app API-key check is redundant
behind the gateway). If you expose the service `Public`, enable OAuth2 security
on the endpoint in API management. To also use the in-app key check, add
`CALLER_API_KEYS` as a **secret**.

## Egress

The service makes outbound calls to `api.anthropic.com` (classification) and
`dmails.doodadlabs.org` (disposable list). If your data plane enforces egress
control, allow-list both hosts, or the model path degrades to `unknown` and the
list refresh fails (both are logged and non-fatal).

## Health check

Set the liveness/readiness probe to `GET /health` on port 8080 under the
component's DevOps/health-check settings.

## Deploy

Click **Deploy** (builds the image, runs the Dockerfile + Trivy scans, deploys
to Development), then **Promote** to higher environments. Test via the endpoint's
URL:

```bash
curl -s https://<endpoint-url>/classify-email \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@acme.com"}'
```
