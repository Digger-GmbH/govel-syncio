package syncio

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Engine runs mapping and sync jobs against govel-shopify.
type Engine struct {
	Store   *Store
	Shopify *ShopifyClient
}

type variantRef struct {
	ProductID int64
	VariantID int64
	SKU       string
	Barcode   string
	Qty       int32
	Title     string
	Price     string
	Status    string
	Vendor    string
	Type      string
	Tags      string
	BodyHTML  string
}

func indexVariants(products []ShopifyProduct, matchByBarcode bool) map[string]variantRef {
	out := make(map[string]variantRef)
	for _, p := range products {
		for _, v := range p.Variants {
			ref := variantRef{
				ProductID: p.ID,
				VariantID: v.ID,
				SKU:       strings.TrimSpace(v.SKU),
				Barcode:   strings.TrimSpace(v.Barcode),
				Qty:       v.InventoryQuantity,
				Title:     p.Title,
				Price:     v.Price,
				Status:    p.Status,
				Vendor:    p.Vendor,
				Type:      p.ProductType,
				Tags:      p.Tags,
				BodyHTML:  p.BodyHTML,
			}
			if ref.SKU != "" {
				out["sku:"+strings.ToLower(ref.SKU)] = ref
			}
			if matchByBarcode && ref.Barcode != "" {
				out["barcode:"+strings.ToLower(ref.Barcode)] = ref
			}
		}
	}
	return out
}

type MapResult struct {
	Linked          int32
	UnmatchedSource int32
	UnmatchedDest   int32
	Mappings        []ProductMapping
	Notes           string
}

func (e *Engine) MapProductsBySku(ctx context.Context, connectionID, destShopID string, dryRun bool) (*MapResult, error) {
	conn, ok := e.Store.Get(connectionID)
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	dests := conn.DestinationShopIDs
	if destShopID != "" {
		dests = []string{destShopID}
	}
	srcProducts, err := e.Shopify.ListProducts(ctx, conn.SourceShopID, 250)
	if err != nil {
		return nil, fmt.Errorf("list source products: %w", err)
	}
	srcIdx := indexVariants(srcProducts, conn.MatchByBarcode)

	res := &MapResult{Notes: "matched by SKU (barcode fallback when enabled)"}

	for _, destID := range dests {
		destProducts, err := e.Shopify.ListProducts(ctx, destID, 250)
		if err != nil {
			return nil, fmt.Errorf("list dest products %s: %w", destID, err)
		}
		destIdx := indexVariants(destProducts, conn.MatchByBarcode)
		matchedDest := map[string]bool{}
		seenSource := map[string]bool{}

		for key, src := range srcIdx {
			kind := "sku"
			raw := strings.TrimPrefix(key, "sku:")
			if strings.HasPrefix(key, "barcode:") {
				kind = "barcode"
				raw = strings.TrimPrefix(key, "barcode:")
			}
			dest, ok := destIdx[key]
			if !ok {
				continue
			}
			seenSource[key] = true
			matchedDest[key] = true
			m := ProductMapping{
				ConnectionID:      conn.ID,
				DestinationShopID: destID,
				MatchKey:          raw,
				MatchKind:         kind,
				SourceProductID:   strconv.FormatInt(src.ProductID, 10),
				SourceVariantID:   strconv.FormatInt(src.VariantID, 10),
				DestProductID:     strconv.FormatInt(dest.ProductID, 10),
				DestVariantID:     strconv.FormatInt(dest.VariantID, 10),
			}
			if !dryRun {
				saved, err := e.Store.UpsertMapping(m)
				if err != nil {
					return nil, err
				}
				m = saved
			}
			res.Mappings = append(res.Mappings, m)
			res.Linked++
		}

		for key := range srcIdx {
			if !seenSource[key] {
				res.UnmatchedSource++
			}
		}
		for key := range destIdx {
			if !matchedDest[key] {
				res.UnmatchedDest++
			}
		}
	}
	return res, nil
}

type InventoryDelta struct {
	MatchKey          string
	DestinationShopID string
	SourceVariantID   string
	DestVariantID     string
	SourceQty         int32
	DestQty           int32
	PlannedDestQty    int32
	Action            string
	Reason            string
}

type InventoryResult struct {
	Planned int32
	Applied int32
	Skipped int32
	Blocked int32
	Deltas  []InventoryDelta
	Notes   string
}

func (e *Engine) SyncInventory(ctx context.Context, connectionID string, dryRun bool) (*InventoryResult, error) {
	conn, ok := e.Store.Get(connectionID)
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	if !conn.Enabled || !conn.InventorySyncEnabled {
		return &InventoryResult{Notes: "inventory sync disabled on connection"}, nil
	}
	srcProducts, err := e.Shopify.ListProducts(ctx, conn.SourceShopID, 250)
	if err != nil {
		return nil, fmt.Errorf("list source products: %w", err)
	}
	srcIdx := indexVariants(srcProducts, conn.MatchByBarcode)
	res := &InventoryResult{
		Notes: "writes via govel-shopify POST /api/inventory/set (use-case catalog.inventory_write; dest granted_scopes need write_inventory+read_inventory)",
	}

	for _, destID := range conn.DestinationShopIDs {
		if !dryRun {
			if err := e.Shopify.RequireUseCase(ctx, destID, UseCaseInventoryWrite); err != nil {
				res.Blocked++
				res.Deltas = append(res.Deltas, InventoryDelta{
					DestinationShopID: destID,
					Action:            "blocked",
					Reason:            err.Error(),
				})
				continue
			}
		}

		destProducts, err := e.Shopify.ListProducts(ctx, destID, 250)
		if err != nil {
			return nil, fmt.Errorf("list dest products: %w", err)
		}
		destIdx := indexVariants(destProducts, conn.MatchByBarcode)

		mappings := e.Store.ListMappings(conn.ID, destID)
		type pair struct {
			key                string
			src, dest          variantRef
			srcVarID, dstVarID string
		}
		var pairs []pair

		if len(mappings) > 0 {
			for _, m := range mappings {
				sk := m.MatchKind + ":" + strings.ToLower(m.MatchKey)
				src, sok := srcIdx[sk]
				dest, dok := destIdx[sk]
				if !sok || !dok {
					continue
				}
				pairs = append(pairs, pair{key: m.MatchKey, src: src, dest: dest, srcVarID: m.SourceVariantID, dstVarID: m.DestVariantID})
			}
		} else {
			for key, src := range srcIdx {
				dest, ok := destIdx[key]
				if !ok {
					continue
				}
				raw := strings.TrimPrefix(strings.TrimPrefix(key, "sku:"), "barcode:")
				pairs = append(pairs, pair{
					key:      raw,
					src:      src,
					dest:     dest,
					srcVarID: strconv.FormatInt(src.VariantID, 10),
					dstVarID: strconv.FormatInt(dest.VariantID, 10),
				})
			}
		}

		for _, p := range pairs {
			planned := p.dest.Qty
			switch conn.InventoryDirection {
			case "source_to_dest", "two_way":
				planned = p.src.Qty
				if planned < 0 {
					planned = 0
				}
			case "dest_to_source":
				res.Skipped++
				res.Deltas = append(res.Deltas, InventoryDelta{
					MatchKey: p.key, DestinationShopID: destID,
					SourceVariantID: p.srcVarID, DestVariantID: p.dstVarID,
					SourceQty: p.src.Qty, DestQty: p.dest.Qty, PlannedDestQty: p.dest.Qty,
					Action: "skip", Reason: "dest_to_source: destination is authority for this leg",
				})
				continue
			}
			if planned == p.dest.Qty {
				res.Skipped++
				res.Deltas = append(res.Deltas, InventoryDelta{
					MatchKey: p.key, DestinationShopID: destID,
					SourceVariantID: p.srcVarID, DestVariantID: p.dstVarID,
					SourceQty: p.src.Qty, DestQty: p.dest.Qty, PlannedDestQty: planned,
					Action: "skip", Reason: "already in sync",
				})
				continue
			}
			d := InventoryDelta{
				MatchKey: p.key, DestinationShopID: destID,
				SourceVariantID: p.srcVarID, DestVariantID: p.dstVarID,
				SourceQty: p.src.Qty, DestQty: p.dest.Qty, PlannedDestQty: planned,
				Action: "set",
			}
			res.Planned++
			if dryRun {
				d.Reason = "dry_run"
				res.Deltas = append(res.Deltas, d)
				continue
			}
			if err := e.Shopify.SetInventoryQuantity(ctx, destID, p.dstVarID, planned, 0); err != nil {
				d.Action = "blocked"
				d.Reason = err.Error()
				res.Blocked++
				res.Deltas = append(res.Deltas, d)
				continue
			}
			res.Applied++
			res.Deltas = append(res.Deltas, d)
		}
	}
	return res, nil
}

type ProductSyncAction struct {
	MatchKey          string
	DestinationShopID string
	Action            string
	Attributes        []string
	Reason            string
}

type ProductSyncResult struct {
	Planned int32
	Applied int32
	Skipped int32
	Blocked int32
	Actions []ProductSyncAction
	Notes   string
}

func buildAttributePayload(src variantRef, enabledAttrs []string) map[string]any {
	attrs := map[string]any{}
	for _, a := range enabledAttrs {
		switch a {
		case "title":
			attrs["title"] = src.Title
		case "description":
			attrs["body_html"] = src.BodyHTML
		case "price":
			attrs["price"] = src.Price
		case "tags":
			attrs["tags"] = src.Tags
		case "status":
			attrs["status"] = src.Status
		case "vendor":
			attrs["vendor"] = src.Vendor
		case "product_type":
			attrs["product_type"] = src.Type
		case "images", "variants", "metafields":
			// Accepted by govel-shopify as future flags; ignored in v1 write path.
			attrs[a] = true
		}
	}
	return attrs
}

func (e *Engine) SyncProducts(ctx context.Context, connectionID string, dryRun bool) (*ProductSyncResult, error) {
	conn, ok := e.Store.Get(connectionID)
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	if !conn.Enabled || !conn.ProductSyncEnabled {
		return &ProductSyncResult{Notes: "product sync disabled on connection"}, nil
	}
	var enabledAttrs []string
	for _, a := range DefaultProductAttributes {
		if conn.AttributeToggles[a] {
			enabledAttrs = append(enabledAttrs, a)
		}
	}
	if len(enabledAttrs) == 0 && !conn.CreateMissingProducts {
		return &ProductSyncResult{Notes: "no attribute toggles enabled"}, nil
	}

	srcProducts, err := e.Shopify.ListProducts(ctx, conn.SourceShopID, 250)
	if err != nil {
		return nil, err
	}
	srcIdx := indexVariants(srcProducts, conn.MatchByBarcode)
	res := &ProductSyncResult{
		Notes: "updates via PATCH /api/products/{id}; create-missing via POST /api/products (catalog.product_write; dest granted_scopes need write_products+read_products)",
	}

	for _, destID := range conn.DestinationShopIDs {
		if !dryRun {
			if err := e.Shopify.RequireUseCase(ctx, destID, UseCaseProductWrite); err != nil {
				res.Blocked++
				res.Actions = append(res.Actions, ProductSyncAction{
					DestinationShopID: destID,
					Action:            "blocked",
					Reason:            err.Error(),
				})
				continue
			}
		}

		destProducts, err := e.Shopify.ListProducts(ctx, destID, 250)
		if err != nil {
			return nil, fmt.Errorf("list dest products: %w", err)
		}
		destIdx := indexVariants(destProducts, conn.MatchByBarcode)

		mappings := e.Store.ListMappings(conn.ID, destID)
		if len(mappings) == 0 {
			mr, err := e.MapProductsBySku(ctx, conn.ID, destID, dryRun)
			if err != nil {
				return nil, err
			}
			mappings = mr.Mappings
		}

		mappedKeys := map[string]bool{}
		for _, m := range mappings {
			sk := m.MatchKind + ":" + strings.ToLower(m.MatchKey)
			mappedKeys[sk] = true
			src, ok := srcIdx[sk]
			if !ok {
				res.Skipped++
				res.Actions = append(res.Actions, ProductSyncAction{
					MatchKey: m.MatchKey, DestinationShopID: destID,
					Action: "skip", Reason: "source variant missing",
				})
				continue
			}
			if len(enabledAttrs) == 0 {
				continue
			}
			attrs := buildAttributePayload(src, enabledAttrs)
			act := ProductSyncAction{
				MatchKey: m.MatchKey, DestinationShopID: destID,
				Action: "update", Attributes: enabledAttrs,
			}
			res.Planned++
			if dryRun {
				act.Reason = "dry_run"
				res.Actions = append(res.Actions, act)
				continue
			}
			if err := e.Shopify.UpdateProductAttributes(ctx, destID, m.DestProductID, m.DestVariantID, attrs); err != nil {
				act.Action = "blocked"
				act.Reason = err.Error()
				res.Blocked++
				res.Actions = append(res.Actions, act)
				continue
			}
			res.Applied++
			res.Actions = append(res.Actions, act)
		}

		if !conn.CreateMissingProducts {
			continue
		}

		// Prefer SKU keys for create-missing; fall back to barcode-only variants.
		seenCreate := map[string]bool{}
		for key, src := range srcIdx {
			if mappedKeys[key] {
				continue
			}
			if _, exists := destIdx[key]; exists {
				continue
			}
			kind := "sku"
			raw := strings.TrimPrefix(key, "sku:")
			if strings.HasPrefix(key, "barcode:") {
				kind = "barcode"
				raw = strings.TrimPrefix(key, "barcode:")
				// Skip barcode-only create when a SKU key for same variant will run.
				if src.SKU != "" {
					continue
				}
			}
			dedupe := kind + ":" + raw + ":" + strconv.FormatInt(src.VariantID, 10)
			if seenCreate[dedupe] {
				continue
			}
			seenCreate[dedupe] = true

			status := src.Status
			if status == "" {
				status = "draft"
			}
			act := ProductSyncAction{
				MatchKey: raw, DestinationShopID: destID,
				Action: "create", Attributes: enabledAttrs,
			}
			res.Planned++
			if dryRun {
				act.Reason = "dry_run"
				res.Actions = append(res.Actions, act)
				continue
			}
			created, err := e.Shopify.CreateProduct(ctx, destID, CreateProductInput{
				Title:       src.Title,
				BodyHTML:    src.BodyHTML,
				Vendor:      src.Vendor,
				ProductType: src.Type,
				Tags:        src.Tags,
				Status:      status,
				Variants: []CreateVariantInput{{
					Title:   src.Title,
					SKU:     src.SKU,
					Price:   src.Price,
					Barcode: src.Barcode,
				}},
			})
			if err != nil {
				act.Action = "blocked"
				act.Reason = err.Error()
				res.Blocked++
				res.Actions = append(res.Actions, act)
				continue
			}
			destProductID := strconv.FormatInt(created.ID, 10)
			destVariantID := ""
			if len(created.Variants) > 0 {
				destVariantID = strconv.FormatInt(created.Variants[0].ID, 10)
			}
			if _, err := e.Store.UpsertMapping(ProductMapping{
				ConnectionID:      conn.ID,
				DestinationShopID: destID,
				MatchKey:          raw,
				MatchKind:         kind,
				SourceProductID:   strconv.FormatInt(src.ProductID, 10),
				SourceVariantID:   strconv.FormatInt(src.VariantID, 10),
				DestProductID:     destProductID,
				DestVariantID:     destVariantID,
			}); err != nil {
				act.Reason = "created but mapping failed: " + err.Error()
			}
			res.Applied++
			res.Actions = append(res.Actions, act)
		}
	}
	return res, nil
}
