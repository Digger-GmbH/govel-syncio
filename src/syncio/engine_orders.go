package syncio

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type OrderForwardAction struct {
	DestinationShopID string `json:"destination_shop_id"`
	DestOrderID       string `json:"dest_order_id"`
	DestOrderName     string `json:"dest_order_name,omitempty"`
	SourceOrderID     string `json:"source_order_id,omitempty"`
	SourceOrderName   string `json:"source_order_name,omitempty"`
	Action            string `json:"action"` // forward | skip | blocked
	Reason            string `json:"reason,omitempty"`
}

type OrderForwardResult struct {
	Planned int32                `json:"planned"`
	Applied int32                `json:"applied"`
	Skipped int32                `json:"skipped"`
	Blocked int32                `json:"blocked"`
	Actions []OrderForwardAction `json:"actions"`
	Notes   string               `json:"notes"`
}

type FulfillmentSyncAction struct {
	DestinationShopID string `json:"destination_shop_id"`
	DestOrderID       string `json:"dest_order_id"`
	SourceOrderID     string `json:"source_order_id"`
	TrackingNumber    string `json:"tracking_number,omitempty"`
	TrackingCompany   string `json:"tracking_company,omitempty"`
	Action            string `json:"action"` // sync | skip | blocked
	Reason            string `json:"reason,omitempty"`
}

type FulfillmentSyncResult struct {
	Planned int32                   `json:"planned"`
	Applied int32                   `json:"applied"`
	Skipped int32                   `json:"skipped"`
	Blocked int32                   `json:"blocked"`
	Actions []FulfillmentSyncAction `json:"actions"`
	Notes   string                  `json:"notes"`
}

func destVariantToSource(mappings []ProductMapping) map[string]string {
	out := map[string]string{}
	for _, m := range mappings {
		if m.DestVariantID == "" || m.SourceVariantID == "" {
			continue
		}
		out[m.DestVariantID] = m.SourceVariantID
	}
	return out
}

func isPaidFinancial(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paid", "partially_paid":
		return true
	default:
		return false
	}
}

// ForwardOrders creates equivalent paid destination orders on the source shop for fulfillment.
func (e *Engine) ForwardOrders(ctx context.Context, connectionID string, dryRun bool) (*OrderForwardResult, error) {
	conn, ok := e.Store.Get(connectionID)
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	res := &OrderForwardResult{
		Notes: "dest paid/open orders → source CreateOrder (orders.write); link via govel_syncio_order_mapping + note_attributes",
	}
	if !conn.Enabled || !conn.OrderForwardingEnabled {
		res.Notes = "order forwarding disabled on connection"
		return res, nil
	}
	if !dryRun {
		if err := e.Shopify.RequireUseCase(ctx, conn.SourceShopID, UseCaseOrdersWrite); err != nil {
			return nil, err
		}
	}

	for _, destID := range conn.DestinationShopIDs {
		orders, err := e.Shopify.ListOrders(ctx, destID, "open", "paid", 50)
		if err != nil {
			return nil, fmt.Errorf("list dest orders %s: %w", destID, err)
		}
		// Also pick up partially_paid open orders.
		partial, err := e.Shopify.ListOrders(ctx, destID, "open", "partially_paid", 50)
		if err != nil {
			return nil, fmt.Errorf("list dest partial orders %s: %w", destID, err)
		}
		orders = append(orders, partial...)

		variantMap := destVariantToSource(e.Store.ListMappings(conn.ID, destID))

		seen := map[int64]struct{}{}
		for _, o := range orders {
			if _, dup := seen[o.ID]; dup {
				continue
			}
			seen[o.ID] = struct{}{}
			destOrderID := strconv.FormatInt(o.ID, 10)
			action := OrderForwardAction{
				DestinationShopID: destID,
				DestOrderID:       destOrderID,
				DestOrderName:     o.Name,
			}
			if !isPaidFinancial(o.FinancialStatus) {
				action.Action = "skip"
				action.Reason = "financial_status not paid/partially_paid"
				res.Skipped++
				res.Actions = append(res.Actions, action)
				continue
			}
			if existing, ok := e.Store.FindOrderMapping(conn.ID, destID, destOrderID); ok && existing.SourceOrderID != "" {
				action.Action = "skip"
				action.Reason = "already forwarded"
				action.SourceOrderID = existing.SourceOrderID
				action.SourceOrderName = existing.SourceOrderName
				res.Skipped++
				res.Actions = append(res.Actions, action)
				continue
			}
			if len(o.LineItems) == 0 {
				action.Action = "blocked"
				action.Reason = "no line items"
				res.Blocked++
				res.Actions = append(res.Actions, action)
				continue
			}

			lineItems := make([]CreateOrderLineItem, 0, len(o.LineItems))
			unmapped := []string{}
			for _, li := range o.LineItems {
				destVar := strconv.FormatInt(li.VariantID, 10)
				srcVar, ok := variantMap[destVar]
				if !ok || srcVar == "" {
					unmapped = append(unmapped, destVar)
					continue
				}
				srcID, err := strconv.ParseInt(srcVar, 10, 64)
				if err != nil || srcID == 0 {
					unmapped = append(unmapped, destVar)
					continue
				}
				qty := li.Quantity
				if qty <= 0 {
					qty = 1
				}
				lineItems = append(lineItems, CreateOrderLineItem{
					VariantID: srcID, Quantity: qty, Title: li.Title, Price: li.Price, SKU: li.SKU,
				})
			}
			if len(unmapped) > 0 {
				action.Action = "blocked"
				action.Reason = "unmapped dest variants: " + strings.Join(unmapped, ",")
				res.Blocked++
				res.Actions = append(res.Actions, action)
				continue
			}

			res.Planned++
			action.Action = "forward"
			note := strings.TrimSpace(o.Note)
			fwdNote := fmt.Sprintf("Syncio forward from dest order %s (%s shop %s)", o.Name, destOrderID, destID)
			if note != "" {
				fwdNote = note + "\n" + fwdNote
			}
			tags := "syncio,forwarded"
			if strings.TrimSpace(o.Tags) != "" {
				tags = o.Tags + ",syncio,forwarded"
			}
			if dryRun {
				action.Reason = "dry_run"
				res.Actions = append(res.Actions, action)
				continue
			}
			created, err := e.Shopify.CreateOrder(ctx, conn.SourceShopID, CreateOrderInput{
				Email: o.Email, FinancialStatus: "paid", Note: fwdNote, Tags: tags,
				InventoryBehaviour: "decrement_ignoring_policy",
				LineItems:          lineItems,
				ShippingAddress:    o.ShippingAddress,
				BillingAddress:     o.BillingAddress,
				NoteAttributes: []NoteAttribute{
					{Name: "syncio_dest_order_id", Value: destOrderID},
					{Name: "syncio_dest_order_name", Value: o.Name},
					{Name: "syncio_dest_shop_id", Value: destID},
					{Name: "syncio_connection_id", Value: conn.ID},
				},
			})
			if err != nil {
				action.Action = "blocked"
				action.Reason = err.Error()
				res.Blocked++
				res.Actions = append(res.Actions, action)
				continue
			}
			srcID := strconv.FormatInt(created.ID, 10)
			action.SourceOrderID = srcID
			action.SourceOrderName = created.Name
			if _, err := e.Store.UpsertOrderMapping(OrderMapping{
				ConnectionID: conn.ID, DestinationShopID: destID,
				DestOrderID: destOrderID, DestOrderName: o.Name,
				SourceOrderID: srcID, SourceOrderName: created.Name,
			}); err != nil {
				action.Action = "blocked"
				action.Reason = "created source order but mapping save failed: " + err.Error()
				res.Blocked++
				res.Actions = append(res.Actions, action)
				continue
			}
			res.Applied++
			res.Actions = append(res.Actions, action)
		}
	}
	return res, nil
}

// SyncFulfillments copies source tracking/fulfillment status onto mapped destination orders.
func (e *Engine) SyncFulfillments(ctx context.Context, connectionID string, dryRun bool) (*FulfillmentSyncResult, error) {
	conn, ok := e.Store.Get(connectionID)
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	res := &FulfillmentSyncResult{
		Notes: "source fulfillments → dest CreateFulfillment / UpdateFulfillmentTracking (orders.fulfillment_write)",
	}
	if !conn.Enabled || !conn.OrderForwardingEnabled {
		res.Notes = "order forwarding disabled on connection"
		return res, nil
	}

	mappings := e.Store.ListOrderMappings(conn.ID, "")
	for _, m := range mappings {
		action := FulfillmentSyncAction{
			DestinationShopID: m.DestinationShopID,
			DestOrderID:       m.DestOrderID,
			SourceOrderID:     m.SourceOrderID,
		}
		if m.SourceOrderID == "" || m.DestOrderID == "" {
			action.Action = "skip"
			action.Reason = "incomplete mapping"
			res.Skipped++
			res.Actions = append(res.Actions, action)
			continue
		}
		srcOrderID, err := strconv.ParseInt(m.SourceOrderID, 10, 64)
		if err != nil || srcOrderID == 0 {
			action.Action = "blocked"
			action.Reason = "invalid source_order_id"
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}
		destOrderID, err := strconv.ParseInt(m.DestOrderID, 10, 64)
		if err != nil || destOrderID == 0 {
			action.Action = "blocked"
			action.Reason = "invalid dest_order_id"
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}

		srcFulfillments, err := e.Shopify.ListFulfillments(ctx, conn.SourceShopID, srcOrderID)
		if err != nil {
			action.Action = "blocked"
			action.Reason = err.Error()
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}
		var best *ShopifyFulfillment
		for i := range srcFulfillments {
			f := &srcFulfillments[i]
			if strings.TrimSpace(f.TrackingNumber) == "" && strings.TrimSpace(f.TrackingURL) == "" {
				continue
			}
			best = f
		}
		if best == nil {
			action.Action = "skip"
			action.Reason = "no source tracking yet"
			res.Skipped++
			res.Actions = append(res.Actions, action)
			continue
		}
		action.TrackingNumber = best.TrackingNumber
		action.TrackingCompany = best.TrackingCompany
		if m.LastTrackingNumber != "" && m.LastTrackingNumber == best.TrackingNumber {
			action.Action = "skip"
			action.Reason = "tracking already synced"
			res.Skipped++
			res.Actions = append(res.Actions, action)
			continue
		}

		res.Planned++
		action.Action = "sync"
		if dryRun {
			action.Reason = "dry_run"
			res.Actions = append(res.Actions, action)
			continue
		}
		if err := e.Shopify.RequireUseCase(ctx, m.DestinationShopID, UseCaseFulfillmentWrite); err != nil {
			action.Action = "blocked"
			action.Reason = err.Error()
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}

		destFulfillments, err := e.Shopify.ListFulfillments(ctx, m.DestinationShopID, destOrderID)
		if err != nil {
			action.Action = "blocked"
			action.Reason = err.Error()
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}
		trackIn := CreateFulfillmentInput{
			NotifyCustomer:  false,
			TrackingCompany: best.TrackingCompany,
			TrackingNumber:  best.TrackingNumber,
			TrackingURL:     best.TrackingURL,
		}
		var syncErr error
		if len(destFulfillments) > 0 {
			_, syncErr = e.Shopify.UpdateFulfillmentTracking(ctx, m.DestinationShopID, destFulfillments[0].ID, trackIn)
		} else {
			_, syncErr = e.Shopify.CreateFulfillment(ctx, m.DestinationShopID, destOrderID, trackIn)
		}
		if syncErr != nil {
			action.Action = "blocked"
			action.Reason = syncErr.Error()
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}
		now := time.Now().UTC()
		m.LastTrackingNumber = best.TrackingNumber
		m.LastFulfillmentSync = &now
		if _, err := e.Store.UpsertOrderMapping(m); err != nil {
			action.Action = "blocked"
			action.Reason = "synced but mapping update failed: " + err.Error()
			res.Blocked++
			res.Actions = append(res.Actions, action)
			continue
		}
		res.Applied++
		res.Actions = append(res.Actions, action)
	}
	return res, nil
}
