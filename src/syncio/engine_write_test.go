package syncio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncInventoryAppliesSet(t *testing.T) {
	var setCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/shops/dest/capabilities":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"capabilities": []map[string]any{{
					"use_case_id": "catalog.inventory_write", "satisfied": true,
				}},
			})
		case r.URL.Path == "/api/products" && r.URL.Query().Get("govel_shop_id") == "src":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"products": []ShopifyProduct{{
					ID: 1, Title: "P",
					Variants: []ShopifyVariant{{ID: 11, SKU: "SKU1", InventoryQuantity: 5}},
				}},
			})
		case r.URL.Path == "/api/products" && r.URL.Query().Get("govel_shop_id") == "dest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"products": []ShopifyProduct{{
					ID: 2, Title: "P",
					Variants: []ShopifyVariant{{ID: 22, SKU: "SKU1", InventoryQuantity: 1}},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/inventory/set":
			setCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	store, err := NewStore(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.Upsert(Connection{
		Name: "c", SourceShopID: "src", DestinationShopIDs: []string{"dest"},
		Enabled: true, InventorySyncEnabled: true, InventoryDirection: "source_to_dest",
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Store: store, Shopify: NewShopifyClient(srv.URL)}
	res, err := engine.SyncInventory(context.Background(), conn.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 1 || setCalls != 1 {
		t.Fatalf("res=%+v setCalls=%d", res, setCalls)
	}
}

func TestSyncProductsCreateMissing(t *testing.T) {
	var created bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/capabilities"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"capabilities": []map[string]any{{
					"use_case_id": "catalog.product_write", "satisfied": true,
				}},
			})
		case r.URL.Path == "/api/products" && r.Method == http.MethodGet && r.URL.Query().Get("govel_shop_id") == "src":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"products": []ShopifyProduct{{
					ID: 1, Title: "OnlySrc", Status: "active",
					Variants: []ShopifyVariant{{ID: 11, SKU: "NEW", Price: "9.00"}},
				}},
			})
		case r.URL.Path == "/api/products" && r.Method == http.MethodGet && r.URL.Query().Get("govel_shop_id") == "dest":
			_ = json.NewEncoder(w).Encode(map[string]any{"products": []ShopifyProduct{}})
		case r.URL.Path == "/api/products" && r.Method == http.MethodPost:
			created = true
			_ = json.NewEncoder(w).Encode(ShopifyProduct{
				ID: 99, Title: "OnlySrc", Variants: []ShopifyVariant{{ID: 88, SKU: "NEW"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	store, err := NewStore(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.Upsert(Connection{
		Name: "c", SourceShopID: "src", DestinationShopIDs: []string{"dest"},
		Enabled: true, ProductSyncEnabled: true, CreateMissingProducts: true,
		AttributeToggles: map[string]bool{"title": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Store: store, Shopify: NewShopifyClient(srv.URL)}
	res, err := engine.SyncProducts(context.Background(), conn.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if !created || res.Applied != 1 {
		t.Fatalf("created=%v res=%+v", created, res)
	}
	maps := store.ListMappings(conn.ID, "dest")
	if len(maps) != 1 || maps[0].DestProductID != "99" {
		t.Fatalf("mappings=%+v", maps)
	}
}
