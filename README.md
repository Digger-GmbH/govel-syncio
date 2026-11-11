# govel-syncio

Govel plugin for **multi-shop Shopify inventory and product sync** (Syncio rebuild).

Shops, OAuth/tokens, and Admin API access live in [`govel-shopify`](https://github.com/Digger-GmbH/govel-shopify). This plugin is a **consumer**: it stores connections (`source_shop_id` ↔ destination shop ids), product mappings, attribute toggles, and Jobs.

**Ownership:** Syncio-specific routes, jobs, UI, and DB stay here — never in `govel_plugin_system`.

Design: Project store `docs/govel-syncio.md` (GV-75).

## Features (v1 scaffold)

- Multi-store connections (one source → N destinations)
- Attribute toggles for product sync (title, description, images, price, tags, variants, status, metafields, …)
- SKU / barcode product mapping job
- Inventory sync job (compare + plan; apply blocked until govel-shopify inventory write API)
- Product sync job (toggled attrs; apply blocked until product write API)
- Admin SPA for connections + toggles + dry-run/run jobs
- Manifest DB schema + `graphql.enabled: true` for host GraphQL / IntraSPA

Deferred: order forwarding, payouts, WooCommerce.

## Requirements

- Go ≥ 1.22
- Running `govel-shopify` reachable at `SHOPIFY_HTTP_BASE`
- Connected shops with scopes for the use-cases you enable

## Configuration

| Variable | Description |
|----------|-------------|
| `GRPC_ADDR` | Default `:50051` |
| `HTTP_ADDR` | Default `:8080` |
| `CONNECTIONS_STORE_PATH` | Default `data/connections.json` |
| `SYNCIO_CONNECTIONS_JSON` | Optional bootstrap JSON |
| `SHOPIFY_HTTP_BASE` | govel-shopify HTTP base (default `http://127.0.0.1:8081`) |

## Run locally

```bash
make proto-all
make tidy
make test
make run
```

Open http://localhost:8080 — create a connection using shop ids from govel-shopify.

## Jobs

Plugin-owned recipes: `docs/fixtures/syncio-jobs.json`. Create via Govel Admin → Jobs after activating this plugin. Core does not seed Syncio jobs.

| RPC | Purpose |
|-----|---------|
| `MapProductsBySku` | Link variants across shops by SKU/barcode |
| `SyncInventory` | Plan/apply stock deltas (oversell clamp to ≥ 0) |
| `SyncProducts` | Push toggled attributes source → destinations |

## Manifest / install

- Plugin id: `govelSyncio`
- Tables: `govel_syncio_connection`, `_destination`, `_attribute_toggle`, `_product_mapping`, `_sync_cursor`
- gRPC: `plugins.govel_syncio.v1.SyncioService` / descriptor `proto/syncio.desc`
- Admin widget: Syncio Connections
- Frontend route: `/connections` (auto-prefixed)

## Blockers (follow-ups in govel-shopify)

1. Generic **inventory write** RPC/HTTP (`POST /api/inventory/set`) + use-case `catalog.inventory_write`
2. Generic **product write/create** APIs + use-case `catalog.product_write`

Until those land, Syncio dry-runs fully; apply paths return `blocked` with a clear reason.

## Docker

```bash
docker build -t govel-syncio .
docker run --rm -p 50051:50051 -p 8080:8080 \
  -e SHOPIFY_HTTP_BASE=http://govel-shopify:8080 \
  -v "$PWD/data:/app/data" \
  govel-syncio
```
