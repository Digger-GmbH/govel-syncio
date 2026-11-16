/*
 * Homolab / operator fixture — NOT executed by plugin install.
 *
 * Canonical Syncio Space / Syncio dashboard seed for product ownership.
 * Core must not seed these rows. Plugin db.sql cannot write core
 * Space/Dashboard/component tables (namespace policy). AdminUIRegister
 * already upserts component_name = SyncioDashboard when plugin id
 * `govelSyncio` is installed. This script wires the Syncio Space +
 * Syncio dashboard to that plugin component.
 *
 * Apply after govelSyncio install (AdminUI assets under
 * /public/plugins/govelSyncio/assets/admin/). Safe to re-run (idempotent).
 *
 * See docs/syncio-space-homolab.md
 */

CREATE SCHEMA IF NOT EXISTS core;
SET search_path TO core, public;

-- One Syncio Space (product decision: not Space-per-connection)
INSERT INTO Space (
    name,
    description,
    is_private,
    is_public,
    status_flags,
    permissions,
    owner_id,
    slug,
    color,
    icon,
    group_id
)
SELECT
    'Syncio',
    'Multi-shop Shopify inventory, product, and order sync (govel-syncio). Connection filters and run-sync live in the dashboard UI.',
    FALSE,
    TRUE,
    128,   -- SpaceStatusSystem
    0,
    0,
    'syncio',
    '#3d9a78',
    'pi-sync',
    '2'
WHERE NOT EXISTS (
    SELECT 1 FROM Space WHERE slug = 'syncio'
);

INSERT INTO Dashboard (
    name,
    description,
    space_id,
    status_flags,
    permissions,
    slug,
    icon
)
SELECT
    'Syncio',
    'Connections, mapping counts, and sync actions from plugin id govelSyncio (SyncioDashboard widget).',
    s.id,
    256,   -- DashboardStatusSystem
    0,
    'syncio',
    'pi-sync'
FROM Space s
WHERE s.slug = 'syncio'
  AND NOT EXISTS (
      SELECT 1 FROM Dashboard WHERE slug = 'syncio'
  );

-- Attach plugin SyncioDashboard (created by AdminUIRegister on install)
INSERT INTO dashboard_component (
    dashboard_id,
    component_id,
    sort_index,
    status_flags,
    permissions,
    width,
    height
)
SELECT
    d.id,
    c.id,
    0,
    256,
    0,
    12,
    10
FROM Dashboard d
CROSS JOIN component c
WHERE d.slug = 'syncio'
  AND c.component_name = 'SyncioDashboard'
  AND NOT EXISTS (
      SELECT 1
      FROM dashboard_component dc
      WHERE dc.dashboard_id = d.id
        AND dc.component_id = c.id
  );

-- Prefer Syncio dashboard as default for the Syncio Space when unset
UPDATE Space s
SET default_dashboard_id = d.id
FROM Dashboard d
WHERE s.slug = 'syncio'
  AND d.slug = 'syncio'
  AND d.space_id = s.id
  AND (s.default_dashboard_id IS NULL OR s.default_dashboard_id = 0);
