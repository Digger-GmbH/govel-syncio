import { createApi, el } from "./api.js";

const emptyForm = () => ({
  name: "",
  source_shop_id: "",
  destination_shop_ids: [],
  enabled: true,
  inventory_sync_enabled: true,
  product_sync_enabled: false,
  inventory_direction: "two_way",
  match_by_barcode: true,
  create_missing_products: false,
  attribute_toggles: {},
});

export function mount(root, { apiBase = "" } = {}) {
  const api = createApi(apiBase);
  const state = {
    shops: [],
    connections: [],
    attributes: [],
    error: "",
    notice: "",
    form: emptyForm(),
    selectedId: "",
    destDraft: "",
  };

  async function refresh() {
    state.error = "";
    try {
      const [shops, conns, attrs] = await Promise.all([
        api.listShops().catch((e) => ({ shops: [], _err: e.message })),
        api.listConnections(),
        api.listAttributes(),
      ]);
      state.shops = shops.shops || [];
      if (shops._err) state.notice = `Shop list unavailable (${shops._err}). Enter govel_shop_id UUIDs manually.`;
      state.connections = conns.connections || [];
      state.attributes = attrs.attributes || [];
      if (!Object.keys(state.form.attribute_toggles || {}).length) {
        const toggles = {};
        for (const a of state.attributes) {
          toggles[a] = a === "title" || a === "price" || a === "status";
        }
        state.form.attribute_toggles = toggles;
      }
    } catch (e) {
      state.error = e.message;
    }
    render();
  }

  function shopLabel(id) {
    const s = state.shops.find((x) => x.id === id);
    return s ? `${s.name} (${s.shop_domain})` : id;
  }

  async function save() {
    state.error = "";
    state.notice = "";
    try {
      const body = {
        ...state.form,
        destination_shop_ids: state.form.destination_shop_ids.filter(Boolean),
      };
      if (state.selectedId) body.id = state.selectedId;
      await api.upsertConnection(body);
      state.form = emptyForm();
      state.selectedId = "";
      await refresh();
      state.notice = "Connection saved";
      render();
    } catch (e) {
      state.error = e.message;
      render();
    }
  }

  function edit(c) {
    state.selectedId = c.id;
    state.form = {
      name: c.name,
      source_shop_id: c.source_shop_id,
      destination_shop_ids: [...(c.destination_shop_ids || [])],
      enabled: !!c.enabled,
      inventory_sync_enabled: !!c.inventory_sync_enabled,
      product_sync_enabled: !!c.product_sync_enabled,
      inventory_direction: c.inventory_direction || "two_way",
      match_by_barcode: !!c.match_by_barcode,
      create_missing_products: !!c.create_missing_products,
      attribute_toggles: { ...(c.attribute_toggles || {}) },
    };
    state.notice = "";
    render();
  }

  async function remove(id) {
    if (!confirm("Delete this connection?")) return;
    try {
      await api.deleteConnection(id);
      if (state.selectedId === id) {
        state.selectedId = "";
        state.form = emptyForm();
      }
      await refresh();
    } catch (e) {
      state.error = e.message;
      render();
    }
  }

  async function runJob(kind, dryRun) {
    if (!state.selectedId) {
      state.error = "Select or save a connection first";
      render();
      return;
    }
    state.error = "";
    state.notice = "";
    try {
      let res;
      if (kind === "map") res = await api.mapProducts({ connection_id: state.selectedId, dry_run: dryRun });
      else if (kind === "inventory") res = await api.syncInventory({ connection_id: state.selectedId, dry_run: dryRun });
      else res = await api.syncProducts({ connection_id: state.selectedId, dry_run: dryRun });
      state.notice = `${kind} → ${JSON.stringify(res)}`;
    } catch (e) {
      state.error = e.message;
    }
    render();
  }

  function shopSelect(value, onChange, placeholder) {
    if (state.shops.length) {
      return el("select", {
        value,
        onChange: (e) => onChange(e.target.value),
      }, [
        el("option", { value: "", text: placeholder || "Select shop…" }),
        ...state.shops.map((s) =>
          el("option", { value: s.id, text: `${s.name} — ${s.shop_domain}`, selected: s.id === value ? "selected" : undefined })
        ),
      ]);
    }
    return el("input", {
      placeholder: placeholder || "govel_shop_id",
      value,
      onInput: (e) => onChange(e.target.value),
    });
  }

  function render() {
    const destList = state.form.destination_shop_ids || [];
    root.replaceChildren(
      el("div", { className: "shell" }, [
        el("div", { className: "header" }, [
          el("div", {}, [
            el("h1", { text: "Syncio Connections" }),
            el("p", { text: "Source ↔ destinations via govel-shopify shop IDs · attribute toggles · inventory/product jobs" }),
          ]),
        ]),
        state.error ? el("p", { className: "error", text: state.error }) : null,
        state.notice ? el("p", { className: "notice", text: state.notice }) : null,
        el("div", { className: "panel" }, [
          el("h2", { text: state.selectedId ? "Edit connection" : "New connection" }),
          el("div", { className: "form-row" }, [
            el("label", { text: "Name" }),
            el("input", {
              value: state.form.name,
              onInput: (e) => { state.form.name = e.target.value; },
            }),
          ]),
          el("div", { className: "form-row" }, [
            el("label", { text: "Source shop" }),
            shopSelect(state.form.source_shop_id, (v) => { state.form.source_shop_id = v; }, "Source govel_shop_id"),
          ]),
          el("div", { className: "form-row" }, [
            el("label", { text: "Destinations" }),
            el("div", { className: "dest-row" }, [
              shopSelect(state.destDraft, (v) => { state.destDraft = v; }, "Add destination"),
              el("button", {
                text: "Add",
                onClick: () => {
                  if (state.destDraft && !destList.includes(state.destDraft)) {
                    state.form.destination_shop_ids = [...destList, state.destDraft];
                    state.destDraft = "";
                    render();
                  }
                },
              }),
            ]),
            el("ul", { className: "chip-list" }, destList.map((id) =>
              el("li", {}, [
                el("span", { text: shopLabel(id) }),
                el("button", {
                  text: "×",
                  onClick: () => {
                    state.form.destination_shop_ids = destList.filter((x) => x !== id);
                    render();
                  },
                }),
              ])
            )),
          ]),
          el("div", { className: "form-row checks" }, [
            check("Enabled", state.form.enabled, (v) => { state.form.enabled = v; }),
            check("Inventory sync", state.form.inventory_sync_enabled, (v) => { state.form.inventory_sync_enabled = v; }),
            check("Product sync", state.form.product_sync_enabled, (v) => { state.form.product_sync_enabled = v; }),
            check("Match barcode", state.form.match_by_barcode, (v) => { state.form.match_by_barcode = v; }),
            check("Create missing", state.form.create_missing_products, (v) => { state.form.create_missing_products = v; }),
          ]),
          el("div", { className: "form-row" }, [
            el("label", { text: "Inventory direction" }),
            el("select", {
              value: state.form.inventory_direction,
              onChange: (e) => { state.form.inventory_direction = e.target.value; },
            }, ["two_way", "source_to_dest", "dest_to_source"].map((d) =>
              el("option", { value: d, text: d, selected: d === state.form.inventory_direction ? "selected" : undefined })
            )),
          ]),
          el("div", { className: "form-row" }, [
            el("label", { text: "Product attribute toggles" }),
            el("div", { className: "toggles" }, (state.attributes.length ? state.attributes : Object.keys(state.form.attribute_toggles || {})).map((a) =>
              check(a, !!state.form.attribute_toggles[a], (v) => {
                state.form.attribute_toggles = { ...state.form.attribute_toggles, [a]: v };
              })
            )),
          ]),
          el("div", { className: "actions" }, [
            el("button", { className: "primary", text: "Save connection", onClick: save }),
            state.selectedId
              ? el("button", {
                  text: "Clear",
                  onClick: () => {
                    state.selectedId = "";
                    state.form = emptyForm();
                    render();
                  },
                })
              : null,
          ]),
          state.selectedId
            ? el("div", { className: "actions" }, [
                el("button", { text: "Map SKUs (dry)", onClick: () => runJob("map", true) }),
                el("button", { text: "Map SKU", onClick: () => runJob("map", false) }),
                el("button", { text: "Inventory dry-run", onClick: () => runJob("inventory", true) }),
                el("button", { text: "Sync inventory", onClick: () => runJob("inventory", false) }),
                el("button", { text: "Products dry-run", onClick: () => runJob("products", true) }),
                el("button", { text: "Sync products", onClick: () => runJob("products", false) }),
              ])
            : null,
        ]),
        el("table", { className: "table" }, [
          el("thead", {}, [
            el("tr", {}, ["Name", "Source", "Destinations", "Inv", "Prod", ""].map((t) => el("th", { text: t }))),
          ]),
          el("tbody", {}, state.connections.map((c) =>
            el("tr", {}, [
              el("td", { text: c.name }),
              el("td", { text: shopLabel(c.source_shop_id) }),
              el("td", { text: (c.destination_shop_ids || []).map(shopLabel).join(", ") || "—" }),
              el("td", { text: c.inventory_sync_enabled ? c.inventory_direction : "off" }),
              el("td", { text: c.product_sync_enabled ? "on" : "off" }),
              el("td", {}, [
                el("button", { text: "Edit", onClick: () => edit(c) }),
                el("button", { text: "Delete", onClick: () => remove(c.id) }),
              ]),
            ])
          )),
        ]),
      ])
    );
  }

  function check(label, checked, onChange) {
    return el("label", { className: "check" }, [
      el("input", {
        type: "checkbox",
        checked,
        onChange: (e) => {
          onChange(e.target.checked);
          render();
        },
      }),
      el("span", { text: label }),
    ]);
  }

  refresh();
}
