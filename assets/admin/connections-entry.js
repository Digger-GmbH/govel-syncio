/**
 * Syncio Connections CRUD — Govel dashboard embed entry for plugin id `govelSyncio`.
 *
 * DashboardRenderer loads plugin widgets via classic <script> (not type=module)
 * and calls window[entryFunction](containerId, props). This IIFE registers
 * `mountSyncioConnections` and dynamically imports the ESM CRUD SPA.
 *
 * Assets land at /public/plugins/govelSyncio/assets/admin/ after AdminUIAction.
 *
 * IMPORTANT: Do NOT register this mount via registerGovelComponent /
 * registerPluginComponent — DashboardRenderer Strategy 1 would treat it as React
 * and leave a blank white widget (same pitfall as SyncioDashboard / Chatwoot).
 *
 * Day-to-day path for Max (no SYNCIO_CONNECTIONS_JSON required):
 *   https://govel.dev/plugins/govelSyncio/
 * or attach widget "Syncio Connections" on any dashboard.
 * Admin → Plugins → govelSyncio settings is install/env/lifecycle only — not CRUD.
 */
(function (global) {
  "use strict";

  var ASSET_BASE = "/public/plugins/govelSyncio/assets/admin";
  var ASSET_VERSION = "0.2.2-connections-cta";
  var DEFAULT_API_BASE = "/plugins/govelSyncio";

  function resolveRoot(containerId) {
    if (!containerId) return null;
    if (typeof containerId === "string") {
      return document.getElementById(containerId);
    }
    if (containerId.nodeType === 1) return containerId;
    return null;
  }

  function showMessage(root, title, body, isError) {
    var border = isError ? "#f5c6cb" : "#99f6e4";
    var bg = isError ? "#f8d7da" : "#f0fdfa";
    var color = isError ? "#721c24" : "#0f766e";
    root.innerHTML =
      '<section class="sx-conn-msg" style="padding:1.25rem;border:1px solid ' +
      border +
      ";background:" +
      bg +
      ";border-radius:6px;font-family:system-ui,sans-serif;color:" +
      color +
      ';">' +
      '<strong style="display:block;margin-bottom:0.35rem;">' +
      title +
      "</strong>" +
      '<p style="margin:0;line-height:1.45;max-width:40rem;">' +
      body +
      "</p></section>";
  }

  function resolveApiBase(props) {
    if (props && props.apiBase) return String(props.apiBase);
    if (global.__GOVEL_SYNCIO_API__) return String(global.__GOVEL_SYNCIO_API__);
    return DEFAULT_API_BASE;
  }

  function ensureAdminCss() {
    var href = ASSET_BASE + "/govel-syncio-admin.css?v=" + ASSET_VERSION;
    if (document.querySelector('link[data-sx-admin-css="1"]')) return;
    var existing = document.querySelector('link[href*="govel-syncio-admin.css"]');
    if (existing) {
      existing.href = href;
      existing.setAttribute("data-sx-admin-css", "1");
      return;
    }
    var link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    link.setAttribute("data-sx-admin-css", "1");
    document.head.appendChild(link);
  }

  function mountAsync(containerId, props) {
    var root = resolveRoot(containerId);
    if (!root) {
      console.error("mountSyncioConnections: container not found", containerId);
      return Promise.resolve(false);
    }

    ensureAdminCss();
    showMessage(root, "Syncio Connections", "Loading connection editor…", false);

    return import(ASSET_BASE + "/connections.js?v=" + ASSET_VERSION)
      .then(function (mod) {
        var mount = mod && (mod.mount || (mod.default && mod.default.mount));
        if (typeof mount !== "function") {
          throw new Error("connections.js does not export mount");
        }
        root.innerHTML = "";
        root.classList.add("sx-connections-host");
        root.style.height = "100%";
        root.style.minHeight = "0";
        root.style.overflow = "auto";
        var opts = Object.assign({}, props || {}, {
          apiBase: resolveApiBase(props),
          componentName: "Connections",
        });
        mount(root, opts);
        root.setAttribute("data-component-initialized", "true");
        root.setAttribute("data-plugin-component", "Connections");
        return true;
      })
      .catch(function (err) {
        console.error("mountSyncioConnections failed:", err);
        showMessage(
          root,
          "Syncio Connections konnte nicht geladen werden",
          "Plugin assets fehlen unter " +
            ASSET_BASE +
            " oder die Syncio-API ist nicht erreichbar. " +
            "Open <code>" +
            DEFAULT_API_BASE +
            "/</code> after login, or re-activate plugin id <code>govelSyncio</code>. " +
            (err && err.message ? "(" + err.message + ")" : ""),
          true
        );
        return false;
      });
  }

  /**
   * DashboardRenderer Strategy 2 / 4 entry. Returns true synchronously so the
   * yellow failure banner is not shown; mount continues asynchronously.
   */
  function mountSyncioConnections(containerId, props) {
    mountAsync(containerId, props);
    return true;
  }

  global.mountSyncioConnections = mountSyncioConnections;
})(typeof window !== "undefined" ? window : globalThis);
