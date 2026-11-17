# Dash settings guide

This page explains the settings files in plain English.

## The config folder

When you run Dash with Docker, your computer’s `./config` folder is mounted into the container as `/config`.

That folder holds:

- `app.yaml` or `app.yml` — main settings and machine list
- `apps.d/` — extra app definition files you add
- `data/` — local database for short history charts
- `ssh/` — optional SSH private keys
- `README.txt` — short reminder created on first start

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

After Dash finds a known app (or you add one), put the URL and key in `app.yaml`:

```yaml
    apps:
      - type: homeassistant
        url: http://10.20.20.40:8123
        key: ${HA_TOKEN}
```

Use `${ENV_NAME}` for secrets from environment variables. Do not put secrets in the browser — there is no form for that.

If a key is missing, the card shows a badge telling you to edit the config file.

## Icons

Set `icon:` on a host, app, or block. Dash looks up:

1. [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons)
2. [Simple Icons](https://simpleicons.org/) as a backup

Example: `icon: jellyfin` or `icon: si:github`.

## Caddy and HTTPS

Compose runs **Caddy** in front of Dash:

- Edit `deploy/Caddyfile` with your hostname (default `dash.dns.puvvadi.net`)
- Open ports 80 and 443
- Caddy gets certificates when the name is reachable on the public internet
- For LAN or Tailscale only, use `tls internal` in the Caddyfile (commented example is in the file)

Dash itself is not published on the host. Only Caddy is.

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
