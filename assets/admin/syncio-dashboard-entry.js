/**
 * Syncio Space dashboard — Govel dashboard embed entry for plugin id `govelSyncio`.
 *
 * DashboardRenderer loads plugin widgets via classic <script> (not type=module)
 * and calls window[entryFunction](containerId, props). This IIFE registers
 * `mountSyncioDashboard` and dynamically imports the ESM dashboard.
 *
 * Assets land at /public/plugins/govelSyncio/assets/admin/ after AdminUIAction.
 *
 * IMPORTANT: Do NOT register this mount via registerGovelComponent /
 * registerPluginComponent — DashboardRenderer Strategy 1 would treat it as React
 * and leave a blank white widget (same pitfall as Chatwoot SupportInbox).
 */
(function (global) {
  "use strict";

  var ASSET_BASE = "/public/plugins/govelSyncio/assets/admin";
  var ASSET_VERSION = "0.2.0-space";

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
      '<section class="sx-dash-msg" style="padding:1.25rem;border:1px solid ' +
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
    return "/plugins/govelSyncio";
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
      console.error("mountSyncioDashboard: container not found", containerId);
      return Promise.resolve(false);
    }

    ensureAdminCss();
    showMessage(root, "Syncio", "Loading Syncio dashboard…", false);

    return import(ASSET_BASE + "/syncio-dashboard.js?v=" + ASSET_VERSION)
      .then(function (mod) {
        var mount = mod && (mod.mount || (mod.default && mod.default.mount));
        if (typeof mount !== "function") {
          throw new Error("syncio-dashboard.js does not export mount");
        }
        root.innerHTML = "";
        root.classList.add("sx-dashboard-host");
        root.style.height = "100%";
        root.style.minHeight = "0";
        root.style.overflow = "auto";
        var opts = Object.assign({}, props || {}, {
          apiBase: resolveApiBase(props),
          componentName: "SyncioDashboard",
        });
        mount(root, opts);
        root.setAttribute("data-component-initialized", "true");
        root.setAttribute("data-plugin-component", "SyncioDashboard");
        return true;
      })
      .catch(function (err) {
        console.error("mountSyncioDashboard failed:", err);
        showMessage(
          root,
          "Syncio dashboard konnte nicht geladen werden",
          "Plugin assets fehlen unter " +
            ASSET_BASE +
            " oder die Syncio-API ist nicht erreichbar. " +
            "Plugin id <code>govelSyncio</code> installieren/aktivieren und " +
            "<code>docs/syncio-space-homolab.md</code> folgen. " +
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
  function mountSyncioDashboard(containerId, props) {
    mountAsync(containerId, props);
    return true;
  }

  global.mountSyncioDashboard = mountSyncioDashboard;
})(typeof window !== "undefined" ? window : globalThis);
