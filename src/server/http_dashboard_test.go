package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Digger-GmbH/govel-syncio/src/syncio"
)

func TestHandleDashboardOverview(t *testing.T) {
	dir := t.TempDir()
	store, err := syncio.NewStore(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.Upsert(syncio.Connection{
		Name:                   "Main",
		SourceShopID:           "src",
		DestinationShopIDs:     []string{"dest"},
		Enabled:                true,
		InventorySyncEnabled:   true,
		ProductSyncEnabled:     true,
		OrderForwardingEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	synced := time.Now().UTC().Add(-time.Hour)
	if _, err := store.UpsertMapping(syncio.ProductMapping{
		ConnectionID:      conn.ID,
		DestinationShopID: "dest",
		MatchKey:          "SKU-1",
		MatchKind:         "sku",
		SourceVariantID:   "1",
		DestVariantID:     "2",
		LastSyncedAt:      &synced,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertOrderMapping(syncio.OrderMapping{
		ConnectionID:      conn.ID,
		DestinationShopID: "dest",
		DestOrderID:       "1001",
		SourceOrderID:     "2001",
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHTTPServer(store, nil, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	h.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Summary struct {
			ConnectionCount     int `json:"connection_count"`
			EnabledCount        int `json:"enabled_count"`
			ProductMappingCount int `json:"product_mapping_count"`
			OrderMappingCount   int `json:"order_mapping_count"`
		} `json:"summary"`
		Connections []map[string]any `json:"connections"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Summary.ConnectionCount != 1 || payload.Summary.EnabledCount != 1 {
		t.Fatalf("summary connections: %+v", payload.Summary)
	}
	if payload.Summary.ProductMappingCount != 1 || payload.Summary.OrderMappingCount != 1 {
		t.Fatalf("summary mappings: %+v", payload.Summary)
	}
	if len(payload.Connections) != 1 {
		t.Fatalf("expected 1 connection row, got %d", len(payload.Connections))
	}
	row := payload.Connections[0]
	if row["product_mapping_count"] != float64(1) {
		t.Fatalf("product_mapping_count=%v", row["product_mapping_count"])
	}
	if row["last_product_sync_at"] == nil {
		t.Fatal("expected last_product_sync_at")
	}
}
