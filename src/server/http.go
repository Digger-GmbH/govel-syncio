package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Digger-GmbH/govel-syncio/src/syncio"
)

type HTTPServer struct {
	Store   *syncio.Store
	Engine  *syncio.Engine
	Shopify *syncio.ShopifyClient
	Mux     *http.ServeMux
}

func NewHTTPServer(store *syncio.Store, engine *syncio.Engine, shopify *syncio.ShopifyClient, staticDir string) *HTTPServer {
	h := &HTTPServer{Store: store, Engine: engine, Shopify: shopify, Mux: http.NewServeMux()}
	h.Mux.HandleFunc("/api/health", h.handleHealth)
	h.Mux.HandleFunc("/api/attributes", h.handleAttributes)
	h.Mux.HandleFunc("/api/shops", h.handleShops)
	h.Mux.HandleFunc("/api/connections", h.handleConnections)
	h.Mux.HandleFunc("/api/connections/", h.handleConnectionByID)
	h.Mux.HandleFunc("/api/mappings", h.handleMappings)
	h.Mux.HandleFunc("/api/jobs/map-products", h.handleMapProducts)
	h.Mux.HandleFunc("/api/jobs/sync-inventory", h.handleSyncInventory)
	h.Mux.HandleFunc("/api/jobs/sync-products", h.handleSyncProducts)
	h.Mux.HandleFunc("/api/jobs/forward-orders", h.handleForwardOrders)
	h.Mux.HandleFunc("/api/jobs/sync-fulfillments", h.handleSyncFulfillments)
	h.Mux.HandleFunc("/api/order-mappings", h.handleOrderMappings)
	if staticDir != "" {
		fs := http.FileServer(http.Dir(staticDir))
		h.Mux.Handle("/assets/", http.StripPrefix("/assets/", fs))
		h.Mux.HandleFunc("/", h.handleIndex)
	}
	return h
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func connJSON(c syncio.Connection) map[string]any {
	toggles := c.AttributeToggles
	if toggles == nil {
		toggles = syncio.DefaultAttributeToggles()
	}
	return map[string]any{
		"id":                       c.ID,
		"name":                     c.Name,
		"source_shop_id":           c.SourceShopID,
		"destination_shop_ids":     c.DestinationShopIDs,
		"enabled":                  c.Enabled,
		"inventory_sync_enabled":   c.InventorySyncEnabled,
		"product_sync_enabled":     c.ProductSyncEnabled,
		"order_forwarding_enabled": c.OrderForwardingEnabled,
		"inventory_direction":      c.InventoryDirection,
		"match_by_barcode":         c.MatchByBarcode,
		"create_missing_products":  c.CreateMissingProducts,
		"attribute_toggles":        toggles,
	}
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTPServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html><head><meta charset="utf-8"/><title>Syncio</title>
<link rel="stylesheet" href="/assets/admin/govel-syncio-admin.css"/>
</head><body>
<div id="app"></div>
<script type="module">
import { mount } from "/assets/admin/connections.js";
mount(document.getElementById("app"));
</script>
</body></html>`))
}

func (h *HTTPServer) handleAttributes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attributes": syncio.DefaultProductAttributes})
}

func (h *HTTPServer) handleShops(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	shops, err := h.Shopify.ListShops(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shops": shops})
}

func (h *HTTPServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		list := h.Store.List()
		out := make([]map[string]any, 0, len(list))
		for _, c := range list {
			out = append(out, connJSON(c))
		}
		writeJSON(w, http.StatusOK, map[string]any{"connections": out})
	case http.MethodPost:
		var body syncio.Connection
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		saved, err := h.Store.Upsert(body)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, connJSON(saved))
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleConnectionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/connections/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeErr(w, http.StatusBadRequest, "missing id")
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "attributes" {
		h.handleConnectionAttributes(w, r, id)
		return
	}
	switch r.Method {
	case http.MethodGet:
		c, ok := h.Store.Get(id)
		if !ok {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, connJSON(c))
	case http.MethodDelete:
		if !h.Store.Delete(id) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleConnectionAttributes(w http.ResponseWriter, r *http.Request, id string) {
	c, ok := h.Store.Get(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"connection_id": id, "toggles": c.AttributeToggles})
	case http.MethodPut, http.MethodPost:
		var body struct {
			Toggles map[string]bool `json:"toggles"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Toggles == nil {
			writeErr(w, http.StatusBadRequest, "toggles required")
			return
		}
		c.AttributeToggles = body.Toggles
		saved, err := h.Store.Upsert(c)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"connection_id": id, "toggles": saved.AttributeToggles})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPServer) handleMappings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	q := r.URL.Query()
	list := h.Store.ListMappings(q.Get("connection_id"), q.Get("destination_shop_id"))
	writeJSON(w, http.StatusOK, map[string]any{"mappings": list})
}

func (h *HTTPServer) handleMapProducts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ConnectionID      string `json:"connection_id"`
		DestinationShopID string `json:"destination_shop_id"`
		DryRun            bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.Engine.MapProductsBySku(r.Context(), body.ConnectionID, body.DestinationShopID, body.DryRun)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *HTTPServer) handleSyncInventory(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ConnectionID string `json:"connection_id"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.Engine.SyncInventory(r.Context(), body.ConnectionID, body.DryRun)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *HTTPServer) handleSyncProducts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ConnectionID string `json:"connection_id"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.Engine.SyncProducts(r.Context(), body.ConnectionID, body.DryRun)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *HTTPServer) handleOrderMappings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	list := h.Store.ListOrderMappings(q.Get("connection_id"), q.Get("destination_shop_id"))
	writeJSON(w, http.StatusOK, map[string]any{"mappings": list})
}

func (h *HTTPServer) handleForwardOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ConnectionID string `json:"connection_id"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.Engine.ForwardOrders(r.Context(), body.ConnectionID, body.DryRun)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *HTTPServer) handleSyncFulfillments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ConnectionID string `json:"connection_id"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.Engine.SyncFulfillments(r.Context(), body.ConnectionID, body.DryRun)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
