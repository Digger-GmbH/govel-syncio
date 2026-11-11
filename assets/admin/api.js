export function createApi(apiBase = "") {
  const base = apiBase.replace(/\/$/, "");

  async function request(path, options = {}) {
    const res = await fetch(`${base}${path}`, {
      headers: { "Content-Type": "application/json", ...(options.headers || {}) },
      ...options,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data.error || res.statusText || "request failed");
    }
    return data;
  }

  return {
    listShops: () => request("/api/shops"),
    listAttributes: () => request("/api/attributes"),
    listConnections: () => request("/api/connections"),
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
  };
}

export function el(tag, attrs = {}, children = []) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === "className") node.className = v;
    else if (k === "text") node.textContent = v;
    else if (k === "checked") node.checked = !!v;
    else if (k === "value") node.value = v;
    else if (k.startsWith("on") && typeof v === "function") node.addEventListener(k.slice(2).toLowerCase(), v);
    else if (v !== undefined && v !== null) node.setAttribute(k, v);
  }
  for (const child of [].concat(children)) {
    if (child == null || child === false) continue;
    node.append(child.nodeType ? child : document.createTextNode(String(child)));
  }
  return node;
}
