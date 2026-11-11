package syncio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetInventoryQuantityPostsBody(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Available":3}`))
	}))
	defer srv.Close()

	c := NewShopifyClient(srv.URL)
	if err := c.SetInventoryQuantity(context.Background(), "shop-1", "99", 7, 0); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/inventory/set" {
		t.Fatalf("got %s %s", gotMethod, gotPath)
	}
	if gotBody["govel_shop_id"] != "shop-1" || gotBody["variant_id"] != "99" {
		t.Fatalf("body=%v", gotBody)
	}
	if int(gotBody["quantity"].(float64)) != 7 {
		t.Fatalf("qty=%v", gotBody["quantity"])
	}
}

func TestUpdateAndCreateProduct(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/products/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ID":1,"Title":"x"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/products":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ID":42,"Title":"New","Variants":[{"ID":7,"SKU":"A"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewShopifyClient(srv.URL)
	if err := c.UpdateProductAttributes(context.Background(), "shop-1", "10", "20", map[string]any{"title": "T"}); err != nil {
		t.Fatal(err)
	}
	created, err := c.CreateProduct(context.Background(), "shop-1", CreateProductInput{
		Title: "New", Variants: []CreateVariantInput{{SKU: "A", Price: "1.00"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 42 || len(created.Variants) != 1 || created.Variants[0].ID != 7 {
		t.Fatalf("created=%+v", created)
	}
	if len(paths) != 2 {
		t.Fatalf("paths=%v", paths)
	}
}

func TestRequireUseCaseMissingScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"capabilities": []map[string]any{{
				"use_case_id":     "catalog.inventory_write",
				"satisfied":       false,
				"missing_scopes":  []string{"write_inventory"},
				"granted_unknown": false,
			}},
		})
	}))
	defer srv.Close()

	c := NewShopifyClient(srv.URL)
	err := c.RequireUseCase(context.Background(), "shop-1", UseCaseInventoryWrite)
	if err == nil || !strings.Contains(err.Error(), "write_inventory") {
		t.Fatalf("err=%v", err)
	}
}
