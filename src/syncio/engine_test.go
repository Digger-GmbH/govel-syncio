package syncio

import (
	"testing"
)

func TestIndexVariantsSKUAndBarcode(t *testing.T) {
	products := []ShopifyProduct{{
		ID: 1,
		Variants: []ShopifyVariant{
			{ID: 10, SKU: "ABC", Barcode: "111", InventoryQuantity: 5},
			{ID: 11, SKU: "", Barcode: "222", InventoryQuantity: 1},
		},
	}}
	idx := indexVariants(products, true)
	if _, ok := idx["sku:abc"]; !ok {
		t.Fatal("missing sku")
	}
	if _, ok := idx["barcode:111"]; !ok {
		t.Fatal("missing barcode")
	}
	if _, ok := idx["barcode:222"]; !ok {
		t.Fatal("missing barcode-only variant")
	}
	idxNo := indexVariants(products, false)
	if _, ok := idxNo["barcode:111"]; ok {
		t.Fatal("barcode should be off")
	}
}
