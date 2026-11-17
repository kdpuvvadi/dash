# Built-in app definitions

These files live in the **binary** (see `internal/apps/bundled/`).

Users do **not** copy or manage them on the server. Dash loads them automatically and matches containers itself.

To change a built-in app as a contributor, edit `internal/apps/bundled/*.yaml` and open a PR.

To add a **custom** app on your own install only, drop a YAML file into `config/apps.d/` (see `docs/add-an-app.md`).
