package syncio

import (
	"path/filepath"
	"testing"
)

func TestStoreUpsertAndMapping(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := store.Upsert(Connection{
		Name:               "HQ → Brand",
		SourceShopID:       "src-1",
		DestinationShopIDs: []string{"dst-1", "dst-2"},
		Enabled:            true,
		InventorySyncEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" {
		t.Fatal("expected id")
	}
	if c.InventoryDirection != "two_way" {
		t.Fatalf("direction=%s", c.InventoryDirection)
	}
	if !c.AttributeToggles["title"] {
		t.Fatal("expected default title toggle")
	}
	got, ok := store.Get(c.ID)
	if !ok || got.Name != "HQ → Brand" {
		t.Fatalf("get: %+v", got)
	}
	m, err := store.UpsertMapping(ProductMapping{
		ConnectionID:      c.ID,
		DestinationShopID: "dst-1",
		MatchKey:          "sku-1",
		MatchKind:         "sku",
		SourceVariantID:   "11",
		DestVariantID:     "22",
	})
	if err != nil {
		t.Fatal(err)
	}
	list := store.ListMappings(c.ID, "dst-1")
	if len(list) != 1 || list[0].ID != m.ID {
		t.Fatalf("mappings: %+v", list)
	}
	if !store.Delete(c.ID) {
		t.Fatal("delete failed")
	}
	if len(store.ListMappings(c.ID, "")) != 0 {
		t.Fatal("mappings should cascade delete")
	}
}
