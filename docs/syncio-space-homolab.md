# Syncio Space dashboard from govel-syncio (Homolab)

**Rule:** Syncio Space / Syncio dashboard UI lives in this plugin — not in `govel_plugin_system` core.

## What the plugin registers

On **install/activate** of plugin id `govelSyncio`, Govel:

1. Copies Admin UI assets → `/public/plugins/govelSyncio/…` (`AdminUIAction`)
2. Upserts `component` row `component_name = SyncioDashboard` with:
   - `entry_function = mountSyncioDashboard`
   - `bundle_type = iife`
   - `dist_path = /public/plugins/govelSyncio/assets/admin`
   - `dist_js_filename = syncio-dashboard-entry.js`
3. Does **not** auto-create Space/Dashboard (manifest has no Space seed; plugin SQL cannot write core tables)

Manifest keys: `adminUi.components[].syncioDashboardComponent`, widget `syncioDashboard`.

Admin Connections (`Connections` / ESM) remains available for CRUD; the Space dashboard is the overview + run-sync surface.

## Homolab / govel.dev install path

### 1. Install or upgrade the plugin

1. Build/push image (Kaniko → in-cluster registry) or re-zip sources including `assets/admin/syncio-dashboard-entry.js`.
2. Admin → Plugins: install/upgrade **`govelSyncio`** (registration/auth only via plugin system).
3. Ensure Admin UI step ran (assets present):

```bash
curl -sI https://govel.dev/public/plugins/govelSyncio/assets/admin/syncio-dashboard-entry.js
# expect 200
curl -sI https://govel.dev/public/plugins/govelSyncio/assets/admin/syncio-dashboard.js
curl -sI https://govel.dev/public/plugins/govelSyncio/assets/admin/api.js
```

4. Confirm component row (Postgres):

```sql
SELECT component_name, entry_function, dist_path, dist_js_filename, bundle_type
FROM core.component
WHERE component_name = 'SyncioDashboard';
```

If missing: re-run plugin finalize / reinstall zip so `AdminUIRegister` runs. Do **not** add a core stub.

### 2. Wire Syncio Space + dashboard

Apply the operator fixture (idempotent) after the component exists — **this is the only supported Space/Dashboard seed**:

```bash
psql "$DATABASE_URL" -f docs/fixtures/syncio-space-dashboard.sql
```

Or equivalent: GraphQL `space_create` (slug `syncio`) → `dashboard_create` → attach component `SyncioDashboard` on the dashboard (same shape as the SQL).

### 3. API base for live data

The IIFE defaults `apiBase` to `/plugins/govelSyncio` (plugin proxy prefix). Override if needed:

```html
<script>window.__GOVEL_SYNCIO_API__ = '/plugins/govelSyncio';</script>
```

Plugin must be **healthy** / reachable via Govel’s plugin proxy. Until then the widget still **loads**; API errors show inside the dashboard.

Required env (Homolab):

- `SHOPIFY_HTTP_BASE=http://govel-govelshopify:8080` (live Service DNS; **not** `govel-shopify`)
- `HTTP_ADDR=:8080`, `GRPC_ADDR=:50051`

**Activate pitfall (known):** `admin_plugin_start` / `ensureDeployment` can wipe Deployment extras (`secretRef`, dual ports 8080+50051, amd64 `nodeSelector`). After Activate, re-attach env/ports/secretRef if Homolab still needs them — do **not** wipe those on purpose when activating.

### 4. Verify

1. Open **Syncio** Space → **Syncio** dashboard (slugs `syncio` / `syncio`).
2. No yellow “Komponente … konnte nicht geladen werden”.
3. Network: `syncio-dashboard-entry.js` 200; then ESM `syncio-dashboard.js` / `api.js`.
4. With healthy plugin + connections: summary cards + connection rows + run-sync buttons.
5. Health: `/plugins/govelSyncio/api/health` → `{"status":"ok"}`.

### Cache-bust after activate

`syncio-dashboard-entry.js` loads ESM/CSS with `?v=` (`ASSET_VERSION`, currently `0.2.0-space`). After plugin **upgrade/activate**, hard-refresh the Syncio Space tab or purge `/public/plugins/govelSyncio/assets/admin/*` if CDN pins stale assets.

## Max: how to open it on govel.dev

| Step | Where |
|------|--------|
| Widget + assets | **govel-syncio** (`SyncioDashboard` / `mountSyncioDashboard`) |
| Install / auth | **govel_plugin_system** Plugin Admin (generic) |
| Space + dashboard rows | **This fixture** or GraphQL (not core seed) |
| Live sync | Plugin healthy + `SHOPIFY_HTTP_BASE` + connections |

1. Log in: [https://govel.dev](https://govel.dev) → Authentik.
2. Space list → **Syncio** (slug `syncio`).
3. Default dashboard: **Syncio** (slug `syncio`).
4. Expect widget **SyncioDashboard** (connections overview, not only Admin → Plugins → connections).

## Homolab deploy steps still needed after merge

1. Build/push `govel-syncio` image with **0.2.0** (or re-zip inbox including new assets).
2. Admin → Plugins → upgrade/activate **`govelSyncio`** — then **restore** any wiped `SHOPIFY_HTTP_BASE` / ports / `nodeSelector` / secretRef if Activate replaced the Deployment.
3. Apply `docs/fixtures/syncio-space-dashboard.sql` against Homolab Postgres.
4. Confirm assets 200 + open Syncio Space on govel.dev.
5. Optional: create Jobs from `docs/fixtures/syncio-jobs.json` if not already present.
