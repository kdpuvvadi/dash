# Dash settings guide

This page explains the settings files in plain English.

## The config folder

When you run Dash with Docker, your computer’s `./config` folder is mounted into the container as `/config`.

That folder holds:

- `app.yaml` or `app.yml` — main settings and machine list (you edit this)
- `secrets.yaml` — optional API keys for apps Dash found (you edit this)
- `apps.d/` — **only** for custom apps Dash does not know yet
- `data/` — local database for short history charts (managed by Dash)
- `ssh/` — optional SSH private keys
- `README.txt` — short reminder created on first start

Built-in app definitions (Home Assistant, Pi-hole, …) are **inside the Dash image**. You do not copy or maintain those files.

The web dashboard is **read-only**. Change settings by editing these files.

## First start

If the config folder has no `app.yaml` / `app.yml`, Dash creates a starter file for you. It watches the local Docker host when possible. You can edit that file anytime.

## Main settings (`app.yaml`)

Common keys:

- `poll_seconds` — how often Dash checks machines
- `history_hours` — how long to keep chart history
- `listen` — address inside the container (default `:8080`). Caddy talks to this; you usually leave it alone
- `alerts.ntfy_url` — optional link for phone/desktop alerts
- `alerts.disk_percent` / `alerts.mem_percent` — when to warn

## Categories, groups, and tags

Use these to keep the board tidy:

- **Category** — big buckets (shown as top tabs), for example Infrastructure or Media
- **Group** — smaller buckets under a category, for example Proxmox or Media stack
- **Tags** — free words for search/filter later

Hosts and apps can set `category`, `group`, and `tags`.

## Layout (rows and columns)

The board is a grid you define:

```yaml
layout:
  sections:
    - id: overview
      title: Your services
      category: overview
      rows:
        - columns: 3
          group: local
          blocks:
            - type: host
              ref: local
```

- Each **row** has `columns: N` — how many equal-width blocks in that row
- **Blocks** fill the row left to right
- Block types: `host`, `guests`, `containers`, `app`, `metric`, `unknown_apps`

If you omit `layout`, the starter file includes a simple default.

## Machines (`hosts`)

Add a machine like this:

```yaml
hosts:
  - name: pve-main
    host: 10.20.20.1
    user: root
    identity_file: /config/ssh/id_ed25519
```

Or watch the Docker host Dash runs on:

```yaml
  - name: local
    local: true
```

For local Docker stats, mount the Docker socket read-only in Compose (see the example in `docker-compose.yml`).

## Apps and API keys

Dash matches common apps from Docker containers automatically. You usually only list **machines** in `app.yaml`.

If an app needs a key, create `secrets.yaml`:

```yaml
homeassistant:
  key: "your-long-lived-token"
pihole:
  key: "${PIHOLE_TOKEN}"
```

You can still override a specific app under a host with an `apps:` block if you need a custom URL. That is optional.

If a key is missing, the card shows a badge telling you to edit `secrets.yaml`.

## Icons

Set `icon:` on a host, app, or block. Dash looks up:

1. [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons)
2. [Simple Icons](https://simpleicons.org/) as a backup

Example: `icon: jellyfin` or `icon: si:github`.

## Caddy and HTTPS

Compose runs **caddybuilds/caddy-cloudflare** in front of Dash:

- `Caddyfile` next to `compose.yaml` (hostname default `dash.dns.puvvadi.net`)
- Ports 80 and 443 only (Dash has no host port)
- Certificates use **Cloudflare DNS** verification — set `CLOUDFLARE_API_TOKEN` in `.env`
- Optional `CADDY_EMAIL` in `.env` for Let’s Encrypt contact

The app image is pulled from GitHub Container Registry (`ghcr.io/.../dash`). The server folder should not hold source code or a local binary.

## Unknown apps

If a container does not match any app definition, it shows under **Unknown apps**. You can:

1. Request support (GitHub issue template)
2. Add a local file under `apps.d/` (see `docs/add-an-app.md`)
3. Send a pull request with a new file under `apps/`

## Checking your config

```bash
dash validate -config ./config
dash apps -config ./config
```
