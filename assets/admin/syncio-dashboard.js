/**
 * Syncio Space dashboard — connections overview, mapping counts, run-sync actions.
 * Mounted by syncio-dashboard-entry.js (IIFE) for Govel DashboardRenderer.
 */
import { createApi, el, formatTs } from "./api.js";

const DEFAULT_API_BASE = "/plugins/govelSyncio";
const CONNECTIONS_URL = "/plugins/govelSyncio/";

/** Leave IntraSPA / dashboard host and open the Connections CRUD SPA. */
function openConnections(ev) {
  if (ev && typeof ev.preventDefault === "function") ev.preventDefault();
  if (ev && typeof ev.stopPropagation === "function") ev.stopPropagation();
  const target =
    (typeof window !== "undefined" && (window.top || window)) || null;
  if (target && target.location) {
    target.location.assign(CONNECTIONS_URL);
    return;
  }
  if (typeof window !== "undefined") {
    window.location.href = CONNECTIONS_URL;
  }
}

export function mount(root, { apiBase } = {}) {
  const resolvedBase =
    (apiBase && String(apiBase)) ||
    (typeof window !== "undefined" && window.__GOVEL_SYNCIO_API__) ||
    DEFAULT_API_BASE;
  const api = createApi(String(resolvedBase));
  const state = {
    summary: {
      connection_count: 0,
      enabled_count: 0,
      product_mapping_count: 0,
      order_mapping_count: 0,
    },
    shops: [],
    connections: [],
    error: "",
    notice: "",
    busyId: "",
  };

  function shopLabel(id) {
    const s = state.shops.find((x) => x.id === id);
    return s ? `${s.name} (${s.shop_domain})` : id || "—";
  }

  function connectionsButton(extraClass) {
    return el("button", {
      type: "button",
      className: "sx-btn sx-btn-primary" + (extraClass ? " " + extraClass : ""),
      text: "Open Connections",
      title: "Open Syncio Connections at /plugins/govelSyncio/",
      onClick: openConnections,
    });
  }

  async function refresh() {
    state.error = "";
    try {
      const [dash, shops] = await Promise.all([
        api.getDashboard(),
        api.listShops().catch(() => ({ shops: [] })),
      ]);
      state.summary = dash.summary || state.summary;
      state.connections = dash.connections || [];
      state.shops = shops.shops || [];
    } catch (e) {
      state.error = e.message || String(e);
    }
    render();
  }

  async function runJob(connectionId, kind, dryRun) {
    state.error = "";
    state.notice = "";
    state.busyId = connectionId + ":" + kind;
    render();
    try {
      const body = { connection_id: connectionId, dry_run: !!dryRun };
      let res;
      if (kind === "map") res = await api.mapProducts(body);
      else if (kind === "inventory") res = await api.syncInventory(body);
      else if (kind === "products") res = await api.syncProducts(body);
      else if (kind === "forward") res = await api.forwardOrders(body);
      else res = await api.syncFulfillments(body);
      const label = dryRun ? `${kind} (dry-run)` : kind;
      state.notice = `${label}: ${summarizeResult(res)}`;
      await refresh();
    } catch (e) {
      state.error = e.message || String(e);
      render();
    } finally {
      state.busyId = "";
      render();
    }
  }

  function summarizeResult(res) {
    if (!res || typeof res !== "object") return String(res);
    const parts = [];
    for (const key of [
      "mapped",
      "unmatched",
      "updated",
      "created",
      "skipped",
      "applied",
      "forwarded",
      "fulfillments",
      "errors",
      "dry_run",
      "notes",
    ]) {
      if (res[key] !== undefined && res[key] !== null && res[key] !== "") {
        parts.push(`${key}=${typeof res[key] === "object" ? JSON.stringify(res[key]) : res[key]}`);
      }
    }
    return parts.length ? parts.join(" · ") : JSON.stringify(res).slice(0, 280);
  }

  function flag(on, label) {
    return el("span", {
      className: "sx-flag" + (on ? " sx-flag-on" : ""),
      text: label + (on ? " on" : " off"),
    });
  }

  function render() {
    const s = state.summary;
    root.replaceChildren(
      el("div", { className: "sx-dashboard" }, [
        el("header", { className: "sx-dash-header" }, [
          el("div", {}, [
            el("h1", { text: "Syncio" }),
            el("p", {
              text: "Multi-shop inventory, product, and order sync via govel-shopify connections.",
            }),
          ]),
          el("div", { className: "sx-dash-actions" }, [
            connectionsButton(),
            el("button", {
              type: "button",
              className: "sx-btn",
              text: "Refresh",
              onClick: () => refresh(),
            }),
          ]),
        ]),
        el("div", { className: "sx-cta" }, [
          el("div", { className: "sx-cta-copy" }, [
            el("strong", { text: "Connections" }),
            el("p", {
              text: "Create and edit source ↔ destination links. Opens /plugins/govelSyncio/ (not Admin → Plugins settings).",
            }),
          ]),
          connectionsButton("sx-cta-btn"),
        ]),
        state.error ? el("p", { className: "sx-error", text: state.error }) : null,
        state.notice ? el("p", { className: "sx-notice", text: state.notice }) : null,
        el("div", { className: "sx-summary" }, [
          summaryCard("Connections", String(s.connection_count || 0), {
            onClick: openConnections,
            title: "Open Connections",
          }),
          summaryCard("Enabled", String(s.enabled_count || 0)),
          summaryCard("Product mappings", String(s.product_mapping_count || 0)),
          summaryCard("Order mappings", String(s.order_mapping_count || 0)),
        ]),
        state.connections.length === 0
          ? el("div", { className: "sx-empty" }, [
              el("strong", { text: "No connections yet" }),
              el("p", {
                text: "Admin → Plugins → govelSyncio is install/env only. Use Open Connections to create a source ↔ destination link.",
              }),
              el("p", {}, [connectionsButton()]),
              el("p", {
                className: "sx-muted",
                text: "SYNCIO_CONNECTIONS_JSON is optional bootstrap only.",
              }),
            ])
          : el("div", { className: "sx-conn-list" }, state.connections.map(renderConnection)),
      ])
    );
  }

  function summaryCard(label, value, opts = {}) {
    const clickable = typeof opts.onClick === "function";
    return el(
      "div",
      {
        className: "sx-summary-card" + (clickable ? " sx-summary-card-action" : ""),
        role: clickable ? "button" : undefined,
        tabIndex: clickable ? "0" : undefined,
        title: opts.title,
        onClick: clickable ? opts.onClick : undefined,
        onKeydown: clickable
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                opts.onClick(e);
              }
            }
          : undefined,
      },
      [
        el("div", { className: "sx-summary-value", text: value }),
        el("div", { className: "sx-summary-label", text: label }),
        clickable
          ? el("div", { className: "sx-summary-hint", text: "Open editor →" })
          : null,
      ]
    );
  }

  function renderConnection(c) {
    const busy = (kind) => state.busyId === c.id + ":" + kind;
    return el("article", { className: "sx-conn" + (c.enabled ? "" : " sx-conn-disabled") }, [
      el("div", { className: "sx-conn-top" }, [
        el("div", {}, [
          el("h2", { text: c.name || "Untitled connection" }),
          el("p", {
            className: "sx-meta",
            text: `Source ${shopLabel(c.source_shop_id)} → ${(c.destination_shop_ids || []).map(shopLabel).join(", ") || "no destinations"}`,
          }),
        ]),
        el("span", {
          className: "sx-status" + (c.enabled ? " sx-status-on" : ""),
          text: c.enabled ? "Enabled" : "Disabled",
        }),
      ]),
      el("div", { className: "sx-flags" }, [
        flag(c.inventory_sync_enabled, "Inventory"),
        flag(c.product_sync_enabled, "Products"),
        flag(c.order_forwarding_enabled, "Orders"),
        c.inventory_sync_enabled
          ? el("span", { className: "sx-flag sx-flag-muted", text: c.inventory_direction || "two_way" })
          : null,
      ]),
      el("div", { className: "sx-stats" }, [
        el("div", {}, [
          el("strong", { text: String(c.product_mapping_count || 0) }),
          el("span", { text: " product maps" }),
        ]),
        el("div", {}, [
          el("strong", { text: String(c.order_mapping_count || 0) }),
          el("span", { text: " order maps" }),
        ]),
        el("div", {}, [
          el("span", { className: "sx-muted", text: "Last product sync " }),
          el("span", { text: formatTs(c.last_product_sync_at) }),
        ]),
        el("div", {}, [
          el("span", { className: "sx-muted", text: "Last fulfillment sync " }),
          el("span", { text: formatTs(c.last_fulfillment_sync_at) }),
        ]),
      ]),
      el("div", { className: "sx-actions" }, [
        actionBtn("Map SKU", () => runJob(c.id, "map", false), busy("map")),
        actionBtn("Inventory", () => runJob(c.id, "inventory", false), busy("inventory")),
        actionBtn("Products", () => runJob(c.id, "products", false), busy("products")),
        actionBtn("Forward orders", () => runJob(c.id, "forward", false), busy("forward")),
        actionBtn("Sync tracking", () => runJob(c.id, "fulfill", false), busy("fulfill")),
        actionBtn("Inventory dry-run", () => runJob(c.id, "inventory", true), busy("inventory")),
      ]),
    ]);
  }

  function actionBtn(label, onClick, isBusy) {
    return el("button", {
      className: "sx-btn" + (isBusy ? " sx-btn-busy" : ""),
      text: isBusy ? "…" : label,
      disabled: isBusy ? "disabled" : undefined,
      onClick: isBusy ? undefined : onClick,
    });
  }

  refresh();
}
