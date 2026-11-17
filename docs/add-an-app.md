# How to add an app

Dash learns about apps from YAML files. You do not write Go or React for a new app.

## Quick path (only on your machine)

1. Copy a file from the project’s `apps/` folder into `config/apps.d/`.
2. Change `id`, `name`, match rules, and API paths.
3. Check the file against `schemas/app-def.schema.json`.
4. In `app.yaml`, add an `apps:` entry with `type`, `url`, and `key` if needed.
5. Wait for reload (or restart the container).

## What the file must include

- `id` — short name used in config
- `name` — label on the board
- `match` — docker image/name/ports so Dash can spot the container
- `collect` and/or `probe` — HTTP calls to read stats
- optional `credentials`, `auth`, `ui`

Invalid files are rejected with a clear error. They are not silently ignored.

## Ask the project to add it

If you do not want to write the file yourself:

1. Open a GitHub **App request** issue (template provided).
2. Include image name, ports, and what numbers you want to see.

## Send a pull request

1. Add the YAML under `apps/`.
2. Run `dash validate`.
3. Open a PR using the app definition checklist.
4. Do not commit real API keys.

## Remember

The dashboard is read-only. Keys and URLs go in config files, not in the web UI.
