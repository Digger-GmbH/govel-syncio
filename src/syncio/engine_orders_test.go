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

func TestForwardOrdersCreatesSourceOrder(t *testing.T) {
	var createdBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/capabilities"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"capabilities": []map[string]any{{
					"use_case_id": "orders.write", "satisfied": true,
				}},
			})
		case r.URL.Path == "/api/orders" && r.Method == http.MethodGet:
			if r.URL.Query().Get("financial_status") == "partially_paid" {
				_ = json.NewEncoder(w).Encode(map[string]any{"orders": []ShopifyOrder{}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"orders": []ShopifyOrder{{
					ID: 1001, Name: "#1001", Email: "a@b.c", FinancialStatus: "paid",
					LineItems: []ShopifyLineItem{{VariantID: 22, Quantity: 2, SKU: "SKU1", Title: "Tee"}},
					ShippingAddress: &ShopifyAddress{Name: "Ada", City: "Berlin", Country: "DE"},
				}},
			})
		case r.URL.Path == "/api/orders" && r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&createdBody)
			_ = json.NewEncoder(w).Encode(ShopifyOrder{ID: 5001, Name: "#S5001", Email: "a@b.c"})
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
		Enabled: true, OrderForwardingEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertMapping(ProductMapping{
		ConnectionID: conn.ID, DestinationShopID: "dest", MatchKey: "SKU1", MatchKind: "sku",
		SourceVariantID: "11", DestVariantID: "22",
	}); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Store: store, Shopify: NewShopifyClient(srv.URL)}
	res, err := engine.ForwardOrders(context.Background(), conn.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 1 || createdBody["govel_shop_id"] != "src" {
		t.Fatalf("res=%+v body=%#v", res, createdBody)
	}
	maps := store.ListOrderMappings(conn.ID, "dest")
	if len(maps) != 1 || maps[0].SourceOrderID != "5001" || maps[0].DestOrderID != "1001" {
		t.Fatalf("mappings=%+v", maps)
	}
}

func TestSyncFulfillmentsCreatesDestFulfillment(t *testing.T) {
	var fulfillCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/capabilities"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"capabilities": []map[string]any{{
					"use_case_id": "orders.fulfillment_write", "satisfied": true,
				}},
			})
		case r.URL.Path == "/api/orders/5001/fulfillments" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"fulfillments": []ShopifyFulfillment{{
					ID: 9, OrderID: 5001, TrackingNumber: "TRACK1", TrackingCompany: "DHL",
				}},
			})
		case r.URL.Path == "/api/orders/1001/fulfillments" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"fulfillments": []ShopifyFulfillment{}})
		case r.URL.Path == "/api/orders/1001/fulfillments" && r.Method == http.MethodPost:
			fulfillCalls++
			_ = json.NewEncoder(w).Encode(ShopifyFulfillment{ID: 42, OrderID: 1001, TrackingNumber: "TRACK1"})
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
		Enabled: true, OrderForwardingEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertOrderMapping(OrderMapping{
		ConnectionID: conn.ID, DestinationShopID: "dest",
		DestOrderID: "1001", SourceOrderID: "5001",
	}); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{Store: store, Shopify: NewShopifyClient(srv.URL)}
	res, err := engine.SyncFulfillments(context.Background(), conn.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 1 || fulfillCalls != 1 {
		t.Fatalf("res=%+v fulfillCalls=%d", res, fulfillCalls)
	}
	maps := store.ListOrderMappings(conn.ID, "dest")
	if len(maps) != 1 || maps[0].LastTrackingNumber != "TRACK1" {
		t.Fatalf("mappings=%+v", maps)
	}
}

func TestForwardOrdersDryRunSkipsWrite(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
		}
		if r.URL.Path == "/api/orders" && r.Method == http.MethodGet {
			if r.URL.Query().Get("financial_status") == "partially_paid" {
				_ = json.NewEncoder(w).Encode(map[string]any{"orders": []ShopifyOrder{}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"orders": []ShopifyOrder{{
					ID: 7, Name: "#7", FinancialStatus: "paid",
					LineItems: []ShopifyLineItem{{VariantID: 22, Quantity: 1}},
				}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	store, err := NewStore(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.Upsert(Connection{
		Name: "c", SourceShopID: "src", DestinationShopIDs: []string{"dest"},
		Enabled: true, OrderForwardingEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = store.UpsertMapping(ProductMapping{
		ConnectionID: conn.ID, DestinationShopID: "dest", MatchKey: "x", MatchKind: "sku",
		SourceVariantID: "11", DestVariantID: "22",
	})
	engine := &Engine{Store: store, Shopify: NewShopifyClient(srv.URL)}
	res, err := engine.ForwardOrders(context.Background(), conn.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Planned != 1 || res.Applied != 0 || posts != 0 {
		t.Fatalf("res=%+v posts=%d", res, posts)
	}
}
