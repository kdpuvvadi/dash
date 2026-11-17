# Contributing to Dash

Thanks for helping. Keep changes small and clear.

## Run locally

**Docker (recommended)**

```bash
docker compose up -d --build
```

Open `https://dash.dns.puvvadi.net` (or your hostname in `deploy/Caddyfile`).

**Dev without Docker**

```bash
# terminal 1 — API
go run ./cmd/dash serve -config ./config

# terminal 2 — UI
cd web && npm install && npm run dev
```

Vite proxies `/api` to port 8080.

## Config and apps

- Settings live under the config folder. See `docs/config.md`.
- New apps are YAML drop-ins. See `docs/add-an-app.md`.
- Request an app with the GitHub issue template, or open a PR under `apps/`.

## Rules

- The web dashboard must stay **read-only** (no config editing in the browser).
- Do not commit secrets.
- App YAML must pass `schemas/app-def.schema.json` and `dash validate`.
- Prefer small commits: one idea per commit.

## Pull requests

- Describe what changed and how you tested it.
- Update docs if settings or schema changed.
- For new app defs, use the app-def PR checklist.
