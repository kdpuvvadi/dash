# Dash

Read-only homelab metrics dashboard.

List machines in a YAML file. Dash connects over SSH (or local Docker), shows Proxmox guests and containers, and can pull stats from known apps using drop-in YAML definitions. The web UI builds itself from your layout config and stays view-only.

## Quick start

Docker Compose: **dash** (no host ports) + **Caddy** (80/443, TLS). No systemd.

On the LXC (as root, home is `/root`):

```bash
cd ~/dash
docker compose up -d --build
```

- App folder: `~/dash/`
- Config: `~/dash/config` → `/config` in the container
- First start creates `app.yaml` if missing
- UI: `https://dash.dns.puvvadi.net` (Caddy; dash is not published on 8080)

Build the linux binary first on your PC (`make release` or `GOOS=linux GOARCH=amd64 go build -o dist/dash ./cmd/dash`), then copy `dist/dash` into `~/dash/dist/` before `docker compose build`. Default Caddyfile uses `tls internal` for LAN/Tailscale; drop that line for public Let's Encrypt.

## Docs

- [Settings guide](docs/config.md) — plain English
- [Add an app](docs/add-an-app.md)
- [Contributing](CONTRIBUTING.md)
- UI target: [docs/design/figma-homelab-overview.png](docs/design/figma-homelab-overview.png)

## Commands

```bash
dash serve -config /config
dash init -config /config
dash validate -config /config
dash apps -config /config
dash discover --host 10.20.20.1
```

## Stack

- Go hub (SSH collector, SQLite history, WebSocket)
- Vite React UI (embedded in the binary)
- Caddy for TLS and reverse proxy
