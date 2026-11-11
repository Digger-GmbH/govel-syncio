package syncio

// DefaultProductAttributes are toggleable fields for product sync (v1).
var DefaultProductAttributes = []string{
	"title",
	"description",
	"images",
	"price",
	"tags",
	"variants",
	"status",
	"metafields",
	"vendor",
	"product_type",
}

// DefaultAttributeToggles returns all attributes disabled except title/price/status.
func DefaultAttributeToggles() map[string]bool {
	out := make(map[string]bool, len(DefaultProductAttributes))
	for _, a := range DefaultProductAttributes {
		out[a] = false
	}
	out["title"] = true
	out["price"] = true
	out["status"] = true
	return out
}
