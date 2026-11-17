# How to add an app

Dash already ships with definitions for common apps (Home Assistant, Pi-hole, Jellyfin, Portainer, qBittorrent, Uptime Kuma, …). Those are built into the image. You do **not** install or edit them.

Dash finds matching containers on your machines and collects stats when it can. If an app needs an API key, put it in `config/secrets.yaml`:

```yaml
homeassistant:
  key: "your-token"
```

## Custom app (only if Dash does not know it)

1. Write a YAML file under `config/apps.d/` (same schema as built-ins).
2. Follow `schemas/app-def.schema.json`.
3. Wait for reload — no rebuild of the image.

You can copy ideas from `internal/apps/bundled/` in the GitHub repo.

## Ask the project to add a built-in

1. Open a GitHub **App request** issue.
2. Include Docker image name, ports, and what numbers you want.

Or send a PR that adds a file under `internal/apps/bundled/`.

## Remember

The dashboard is read-only. Keys go in `secrets.yaml`, not in the browser.
