# Govel Jobs ↔ Syncio

Job definitions live **in this plugin**, not in `govel_plugin_system`.

## Prerequisites

1. Install/activate `govel-shopify` (with catalog write APIs from PR #6 / `docs/catalog-write.md`) and connect shops.
2. On each **destination** shop, set `granted_scopes` to include write scopes for the features you enable:
   - Inventory: `read_inventory`, `write_inventory` (`catalog.inventory_write`)
   - Product sync / create-missing: `read_products`, `write_products` (`catalog.product_write`)
3. Install/activate `govel-syncio`.
4. Create a connection in the Syncio Admin widget (source + destinations = `govel_shop_id`s).

## Recipes

See [`fixtures/syncio-jobs.json`](fixtures/syncio-jobs.json).

Typical flow:

1. `MapProductsBySku` (persist mappings)
2. `SyncInventory` (dry_run first, then apply → `POST /api/inventory/set`)
3. Optional `SyncProducts` (dry_run first; updates `PATCH /api/products/{id}`; create-missing `POST /api/products` when enabled)

Host GraphQL / IntraSPA can read connection/mapping tables declared in `manifest.json` (`graphql.enabled: true`). Side-effectful sync stays on Jobs → gRPC.
