# Govel Jobs ↔ Syncio

Job definitions live **in this plugin**, not in `govel_plugin_system`.

## Prerequisites

1. Install/activate `govel-shopify` and connect shops (tokens + scopes).
2. Install/activate `govel-syncio`.
3. Create a connection in the Syncio Admin widget (source + destinations = `govel_shop_id`s).
4. Prefer shops with `read_products` (+ `read_inventory` / `write_inventory` when available).

## Recipes

See [`fixtures/syncio-jobs.json`](fixtures/syncio-jobs.json).

Typical flow:

1. `MapProductsBySku` (persist mappings)
2. `SyncInventory` (dry_run first)
3. Optional `SyncProducts` when attribute toggles are enabled

Host GraphQL / IntraSPA can read connection/mapping tables declared in `manifest.json` (`graphql.enabled: true`). Side-effectful sync stays on Jobs → gRPC.
