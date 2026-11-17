# Dash

Read-only homelab metrics dashboard.

List machines in a YAML file. Dash connects over SSH (or local Docker), shows Proxmox guests and containers, and can pull stats from known apps using drop-in YAML definitions. The web UI builds itself from your layout config and stays view-only.

## Quick start

**Docker + Caddy (recommended)**

```bash
docker compose up -d --build
```

- Config: `./config` → container `/config`
- First start creates `app.yaml` if missing
- HTTPS: Caddy in front (`deploy/Caddyfile`), default host `dash.dns.puvvadi.net`

**Binary on the dash LXC**

```bash
# copy linux amd64 binary built with: make release
install -m 755 dist/dash /opt/dash/dash
cp -r apps /opt/dash/
cp deploy/dash.service /etc/systemd/system/
systemctl enable --now dash
```

UI: `http://dash.dns.puvvadi.net:8080` (add Caddy later for HTTPS on 443).

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
