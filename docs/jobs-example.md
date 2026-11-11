# Govel Jobs ↔ Syncio

Job definitions live **in this plugin**, not in `govel_plugin_system`.

## Prerequisites

1. Install/activate `govel-shopify` (catalog write + order write APIs; see `docs/catalog-write.md`, `docs/order-write.md`) and connect shops.
2. Set `granted_scopes` for the features you enable:
   - Destinations — inventory: `read_inventory`, `write_inventory`; product: `read_products`, `write_products`; tracking: merchant-managed FO write/read + `read_orders`
   - Source — order forward: `write_orders`, `read_orders` (+ product reads for mapping)
3. Install/activate `govel-syncio`.
4. Create a connection in the Syncio Admin widget (source + destinations = `govel_shop_id`s). Enable **Order forwarding** when needed.

## Recipes

See [`fixtures/syncio-jobs.json`](fixtures/syncio-jobs.json).

Typical flow:

1. `MapProductsBySku` (persist mappings — required before order forward line-item remap)
2. `SyncInventory` (dry_run first, then apply → `POST /api/inventory/set`)
3. Optional `SyncProducts` (dry_run first; updates `PATCH /api/products/{id}`; create-missing `POST /api/products` when enabled)
4. Optional `ForwardOrders` (paid/open dest → source `CreateOrder`; stores `govel_syncio_order_mapping`)
5. Optional `SyncFulfillments` (source tracking → dest fulfill / update tracking)

Host GraphQL / IntraSPA can read connection/mapping tables declared in `manifest.json` (`graphql.enabled: true`). Side-effectful sync stays on Jobs → gRPC.
