# Dash

Read-only homelab metrics dashboard.

List machines in a YAML file. Dash connects over SSH (or local Docker), shows Proxmox guests and containers, and can pull stats from known apps using drop-in YAML definitions. The web UI builds itself from your layout config and stays view-only.

## Server layout (minimal)

On the LXC only keep:

```text
~/dash/
  compose.yaml
  Caddyfile
  .env
  config/
```

No source tree, no binary, no local build. Images are pulled:

- `ghcr.io/<owner>/dash` — app (built by GitHub Actions)
- `caddybuilds/caddy-cloudflare` — Caddy with Cloudflare DNS plugin for TLS

## Quick start on the server

```bash
mkdir -p ~/dash/config
cd ~/dash
# copy compose.yaml, Caddyfile, .env.example→.env from the repo
nano .env   # set CLOUDFLARE_API_TOKEN, GITHUB_OWNER, CADDY_EMAIL

# if the GHCR package is private:
echo "$GHCR_TOKEN" | docker login ghcr.io -u USERNAME --password-stdin

docker compose pull
docker compose up -d
```

- Config: `~/dash/config` → `/config` in the container (first start writes `app.yaml` if missing)
- UI: `https://dash.dns.puvvadi.net`
- TLS: Let’s Encrypt via **Cloudflare DNS** challenge (`CLOUDFLARE_API_TOKEN` needs Zone → DNS → Edit)

## Build / publish (CI)

Push to `main` (or run **Build and push** manually) runs `.github/workflows/publish.yml`:

- Builds the multi-stage `Dockerfile`
- Pushes to `ghcr.io/<github.repository>:latest` (and version tags on `v*`)

## Docs

- [Settings guide](docs/config.md) — plain English
- [Add an app](docs/add-an-app.md)
- [Contributing](CONTRIBUTING.md)
- UI target: [docs/design/figma-homelab-overview.png](docs/design/figma-homelab-overview.png)

## Commands (inside the image / local dev)

```bash
dash serve -config /config
dash init -config /config
dash validate -config /config
dash apps -config /config
dash discover --host 10.20.20.1
```

## Stack

- Go hub (SSH collector, SQLite history, WebSocket)
- Vite React UI (embedded in the image)
- Caddy + Cloudflare DNS for TLS
- GitHub Container Registry for the app image
