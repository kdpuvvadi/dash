# Drop-in app definitions

Put a YAML file here to teach Dash about another app.

1. Copy an example from the `apps/` folder in the project.
2. Change the `id`, match rules, and API paths.
3. Make sure the file follows `schemas/app-def.schema.json`.
4. Restart is usually not needed — Dash reloads this folder.

The web dashboard is read-only. API keys go in `app.yaml`, not in the browser.

If Dash does not know your app, you can open a request issue or send a pull request. See `docs/add-an-app.md`.
