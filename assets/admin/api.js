export function createApi(apiBase = "") {
  const base = apiBase.replace(/\/$/, "");

  function getCsrfToken() {
    try {
      const meta = document.querySelector('meta[name="csrf-token"]');
      if (meta && meta.content) return meta.content;
      if (typeof window !== "undefined" && window.__GOVEL_CSRF_TOKEN__) {
        return String(window.__GOVEL_CSRF_TOKEN__);
      }
      const el = document.querySelector("[data-csrf-token]");
      if (el && el.dataset && el.dataset.csrfToken) return el.dataset.csrfToken;
    } catch (_) {
      /* ignore */
    }
    return "";
  }

  async function request(path, options = {}) {
    const headers = {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    };
    const csrf = getCsrfToken();
    if (csrf) {
      headers["X-CSRF-Token"] = csrf;
    }
    const res = await fetch(`${base}${path}`, {
      credentials: "same-origin",
      ...options,
      headers,
    });
    const raw = await res.text();
    let data = {};
    if (raw) {
      try {
        data = JSON.parse(raw);
      } catch {
        data = {};
      }
    }
    if (!res.ok) {
      const fallback =
        (typeof data.error === "string" && data.error) ||
        (raw && raw.trim().slice(0, 240)) ||
        res.statusText ||
        `request failed (${res.status})`;
      throw new Error(fallback);
    }
    return data;
  }

  return {
    listShops: () => request("/api/shops"),
    listAttributes: () => request("/api/attributes"),
    listConnections: () => request("/api/connections"),
    getDashboard: () => request("/api/dashboard"),
    upsertConnection: (body) =>
      request("/api/connections", { method: "POST", body: JSON.stringify(body) }),
    deleteConnection: (id) => request(`/api/connections/${id}`, { method: "DELETE" }),
    getAttributes: (id) => request(`/api/connections/${id}/attributes`),
    setAttributes: (id, toggles) =>
      request(`/api/connections/${id}/attributes`, {
        method: "PUT",
        body: JSON.stringify({ toggles }),
      }),
    listMappings: (connectionId) =>
      request(`/api/mappings?connection_id=${encodeURIComponent(connectionId || "")}`),
    mapProducts: (body) =>
      request("/api/jobs/map-products", { method: "POST", body: JSON.stringify(body) }),
    syncInventory: (body) =>
      request("/api/jobs/sync-inventory", { method: "POST", body: JSON.stringify(body) }),
    syncProducts: (body) =>
      request("/api/jobs/sync-products", { method: "POST", body: JSON.stringify(body) }),
    forwardOrders: (body) =>
      request("/api/jobs/forward-orders", { method: "POST", body: JSON.stringify(body) }),
    syncFulfillments: (body) =>
      request("/api/jobs/sync-fulfillments", { method: "POST", body: JSON.stringify(body) }),
    listOrderMappings: (connectionId) =>
      request(`/api/order-mappings?connection_id=${encodeURIComponent(connectionId || "")}`),
  };
}

export function el(tag, attrs = {}, children = []) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === "className") node.className = v;
    else if (k === "text") node.textContent = v;
    else if (k === "checked") node.checked = !!v;
    else if (k === "value") node.value = v;
    else if (k === "html") node.innerHTML = v;
    else if (k.startsWith("on") && typeof v === "function") node.addEventListener(k.slice(2).toLowerCase(), v);
    else if (v !== undefined && v !== null) node.setAttribute(k, v);
  }
  for (const child of [].concat(children)) {
    if (child == null || child === false) continue;
    node.append(child.nodeType ? child : document.createTextNode(String(child)));
  }
  return node;
}

export function formatTs(value) {
  if (!value) return "—";
  try {
    return new Date(value).toLocaleString();
  } catch {
    return String(value);
  }
}
