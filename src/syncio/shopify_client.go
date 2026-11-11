package syncio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Required Shopify use-cases (govel-shopify scopes catalog).
const (
	UseCaseInventoryWrite   = "catalog.inventory_write"   // write_inventory + read_inventory
	UseCaseProductWrite     = "catalog.product_write"     // write_products + read_products
	UseCaseOrdersWrite      = "orders.write"              // write_orders + read_orders
	UseCaseFulfillmentWrite = "orders.fulfillment_write"  // merchant-managed FO write + read_orders
)

// ShopifyClient talks to govel-shopify HTTP API (not Shopify Admin directly).
// Write paths target govel-shopify#6 catalog APIs (docs/catalog-write.md).
type ShopifyClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewShopifyClient(base string) *ShopifyClient {
	base = strings.TrimRight(base, "/")
	return &ShopifyClient{
		BaseURL: base,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ShopifyShop struct {
	ID         string   `json:"id"`
	ShopDomain string   `json:"shop_domain"`
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	APIVersion string   `json:"api_version"`
	Scopes     []string `json:"granted_scopes"`
}

// Product/variant views from govel-shopify encode with Go default (PascalCase) field names.
type ShopifyVariant struct {
	ID                int64  `json:"ID"`
	Title             string `json:"Title"`
	SKU               string `json:"SKU"`
	Barcode           string `json:"Barcode"`
	Price             string `json:"Price"`
	InventoryQuantity int32  `json:"InventoryQuantity"`
	InventoryItemID   int64  `json:"InventoryItemID"`
}

type ShopifyProduct struct {
	GovelShopID string           `json:"GovelShopID"`
	ShopDomain  string           `json:"ShopDomain"`
	ID          int64            `json:"ID"`
	Title       string           `json:"Title"`
	Status      string           `json:"Status"`
	Vendor      string           `json:"Vendor"`
	ProductType string           `json:"ProductType"`
	Tags        string           `json:"Tags"`
	BodyHTML    string           `json:"BodyHTML"`
	Variants    []ShopifyVariant `json:"Variants"`
}

type ShopCapability struct {
	UseCaseID      string   `json:"use_case_id"`
	Satisfied      bool     `json:"satisfied"`
	MissingScopes  []string `json:"missing_scopes"`
	GrantedUnknown bool     `json:"granted_unknown"`
}

type CreateProductInput struct {
	Title       string
	BodyHTML    string
	Vendor      string
	ProductType string
	Tags        string
	Status      string
	Variants    []CreateVariantInput
}

type CreateVariantInput struct {
	Title   string
	SKU     string
	Price   string
	Barcode string
}

// Order/fulfillment views from govel-shopify encode with Go default (PascalCase) field names.
type ShopifyAddress struct {
	Name     string `json:"Name"`
	Company  string `json:"Company"`
	Address1 string `json:"Address1"`
	Address2 string `json:"Address2"`
	City     string `json:"City"`
	Province string `json:"Province"`
	Country  string `json:"Country"`
	Zip      string `json:"Zip"`
	Phone    string `json:"Phone"`
}

type ShopifyLineItem struct {
	ID        int64  `json:"ID"`
	Title     string `json:"Title"`
	SKU       string `json:"SKU"`
	Quantity  int32  `json:"Quantity"`
	Price     string `json:"Price"`
	ProductID int64  `json:"ProductID"`
	VariantID int64  `json:"VariantID"`
}

type ShopifyOrder struct {
	GovelShopID       string            `json:"GovelShopID"`
	ShopDomain        string            `json:"ShopDomain"`
	ID                int64             `json:"ID"`
	Name              string            `json:"Name"`
	Email             string            `json:"Email"`
	FinancialStatus   string            `json:"FinancialStatus"`
	FulfillmentStatus string            `json:"FulfillmentStatus"`
	Currency          string            `json:"Currency"`
	Note              string            `json:"Note"`
	Tags              string            `json:"Tags"`
	LineItems         []ShopifyLineItem `json:"LineItems"`
	ShippingAddress   *ShopifyAddress   `json:"ShippingAddress"`
	BillingAddress    *ShopifyAddress   `json:"BillingAddress"`
}

type ShopifyFulfillment struct {
	GovelShopID     string            `json:"GovelShopID"`
	ID              int64             `json:"ID"`
	OrderID         int64             `json:"OrderID"`
	Status          string            `json:"Status"`
	TrackingCompany string            `json:"TrackingCompany"`
	TrackingNumber  string            `json:"TrackingNumber"`
	TrackingURL     string            `json:"TrackingURL"`
	TrackingURLs    []string          `json:"TrackingURLs"`
	LineItems       []ShopifyLineItem `json:"LineItems"`
}

type CreateOrderInput struct {
	Email                  string
	FinancialStatus        string
	Note                   string
	Tags                   string
	InventoryBehaviour     string
	SendReceipt            bool
	SendFulfillmentReceipt bool
	LineItems              []CreateOrderLineItem
	ShippingAddress        *ShopifyAddress
	BillingAddress         *ShopifyAddress
	NoteAttributes         []NoteAttribute
}

type CreateOrderLineItem struct {
	VariantID int64
	Quantity  int32
	Title     string
	Price     string
	SKU       string
}

type NoteAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CreateFulfillmentInput struct {
	NotifyCustomer  bool
	TrackingCompany string
	TrackingNumber  string
	TrackingURL     string
}

func (c *ShopifyClient) doJSON(ctx context.Context, method, path string, q url.Values, reqBody any, dest any) error {
	if c.BaseURL == "" {
		return fmt.Errorf("SHOPIFY_HTTP_BASE is empty")
	}
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rdr io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = res.Status
		}
		// Surface missing-scope / capability errors from govel-shopify as-is.
		return fmt.Errorf("govel-shopify %s %s: %s", method, path, msg)
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func (c *ShopifyClient) ListShops(ctx context.Context) ([]ShopifyShop, error) {
	var payload struct {
		Shops []ShopifyShop `json:"shops"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/shops", nil, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Shops, nil
}

func (c *ShopifyClient) ListProducts(ctx context.Context, govelShopID string, limit int) ([]ShopifyProduct, error) {
	if limit <= 0 {
		limit = 250
	}
	q := url.Values{}
	q.Set("govel_shop_id", govelShopID)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("status", "any")
	var payload struct {
		Products []ShopifyProduct `json:"products"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/products", q, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Products, nil
}

// GetShopCapabilities checks use-cases against the shop's granted_scopes.
// Empty granted_scopes → GrantedUnknown (legacy allow on the shopify side).
func (c *ShopifyClient) GetShopCapabilities(ctx context.Context, govelShopID string, useCaseIDs []string) ([]ShopCapability, error) {
	q := url.Values{}
	for _, id := range useCaseIDs {
		q.Add("use_case_id", id)
	}
	var payload struct {
		Capabilities []ShopCapability `json:"capabilities"`
	}
	path := "/api/shops/" + url.PathEscape(govelShopID) + "/capabilities"
	if err := c.doJSON(ctx, http.MethodGet, path, q, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Capabilities, nil
}

// RequireUseCase fails when the shop declares granted_scopes that do not cover the use-case.
// Legacy shops with empty granted_scopes are allowed (govel-shopify treats them as unknown).
func (c *ShopifyClient) RequireUseCase(ctx context.Context, govelShopID, useCaseID string) error {
	caps, err := c.GetShopCapabilities(ctx, govelShopID, []string{useCaseID})
	if err != nil {
		// If capabilities endpoint is unreachable, let the write call fail with its own error.
		return fmt.Errorf("check capabilities for %s: %w", govelShopID, err)
	}
	for _, cap := range caps {
		if cap.UseCaseID != useCaseID {
			continue
		}
		if cap.GrantedUnknown || cap.Satisfied {
			return nil
		}
		missing := strings.Join(cap.MissingScopes, ", ")
		if missing == "" {
			missing = "unknown"
		}
		return fmt.Errorf("shop %s missing scopes for %s: %s (set granted_scopes on govel-shopify shop)", govelShopID, useCaseID, missing)
	}
	return fmt.Errorf("shop %s: use-case %s not reported by capabilities", govelShopID, useCaseID)
}

// SetInventoryQuantity calls POST /api/inventory/set (catalog.inventory_write).
// locationID 0 → govel-shopify picks the primary/active location.
func (c *ShopifyClient) SetInventoryQuantity(ctx context.Context, govelShopID, variantID string, qty int32, locationID int64) error {
	body := map[string]any{
		"govel_shop_id": govelShopID,
		"variant_id":    variantID,
		"quantity":      qty,
	}
	if locationID > 0 {
		body["location_id"] = locationID
	}
	return c.doJSON(ctx, http.MethodPost, "/api/inventory/set", nil, body, nil)
}

// UpdateProductAttributes calls PATCH /api/products/{id} (catalog.product_write).
func (c *ShopifyClient) UpdateProductAttributes(ctx context.Context, govelShopID, productID, variantID string, attrs map[string]any) error {
	payload := map[string]any{
		"govel_shop_id": govelShopID,
		"attributes":    attrs,
	}
	if variantID != "" {
		payload["variant_id"] = variantID
	}
	path := "/api/products/" + url.PathEscape(productID)
	return c.doJSON(ctx, http.MethodPatch, path, nil, payload, nil)
}

// CreateProduct calls POST /api/products (catalog.product_write) for create-missing flows.
func (c *ShopifyClient) CreateProduct(ctx context.Context, govelShopID string, in CreateProductInput) (*ShopifyProduct, error) {
	body := map[string]any{
		"govel_shop_id": govelShopID,
		"title":         in.Title,
		"body_html":     in.BodyHTML,
		"vendor":        in.Vendor,
		"product_type":  in.ProductType,
		"tags":          in.Tags,
		"status":        in.Status,
	}
	if in.Status == "" {
		body["status"] = "draft"
	}
	variants := make([]map[string]any, 0, len(in.Variants))
	for _, v := range in.Variants {
		variants = append(variants, map[string]any{
			"title":   v.Title,
			"sku":     v.SKU,
			"price":   v.Price,
			"barcode": v.Barcode,
		})
	}
	if len(variants) > 0 {
		body["variants"] = variants
	}
	var out ShopifyProduct
	if err := c.doJSON(ctx, http.MethodPost, "/api/products", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func addressJSON(a *ShopifyAddress) map[string]any {
	if a == nil {
		return nil
	}
	return map[string]any{
		"name": a.Name, "company": a.Company, "address1": a.Address1, "address2": a.Address2,
		"city": a.City, "province": a.Province, "country": a.Country, "zip": a.Zip, "phone": a.Phone,
	}
}

// ListOrders calls GET /api/orders.
func (c *ShopifyClient) ListOrders(ctx context.Context, govelShopID, status, financialStatus string, limit int) ([]ShopifyOrder, error) {
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{}
	q.Set("govel_shop_id", govelShopID)
	q.Set("limit", strconv.Itoa(limit))
	if status != "" {
		q.Set("status", status)
	}
	if financialStatus != "" {
		q.Set("financial_status", financialStatus)
	}
	var payload struct {
		Orders []ShopifyOrder `json:"orders"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/orders", q, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Orders, nil
}

// CreateOrder calls POST /api/orders (orders.write) — typically on the source/fulfillment shop.
func (c *ShopifyClient) CreateOrder(ctx context.Context, govelShopID string, in CreateOrderInput) (*ShopifyOrder, error) {
	body := map[string]any{
		"govel_shop_id":             govelShopID,
		"email":                     in.Email,
		"financial_status":          in.FinancialStatus,
		"note":                      in.Note,
		"tags":                      in.Tags,
		"send_receipt":              in.SendReceipt,
		"send_fulfillment_receipt":  in.SendFulfillmentReceipt,
		"inventory_behaviour":       in.InventoryBehaviour,
	}
	if in.FinancialStatus == "" {
		body["financial_status"] = "paid"
	}
	if in.InventoryBehaviour == "" {
		body["inventory_behaviour"] = "decrement_ignoring_policy"
	}
	items := make([]map[string]any, 0, len(in.LineItems))
	for _, li := range in.LineItems {
		items = append(items, map[string]any{
			"variant_id": li.VariantID,
			"quantity":   li.Quantity,
			"title":      li.Title,
			"price":      li.Price,
			"sku":        li.SKU,
		})
	}
	body["line_items"] = items
	if ship := addressJSON(in.ShippingAddress); ship != nil {
		body["shipping_address"] = ship
	}
	if bill := addressJSON(in.BillingAddress); bill != nil {
		body["billing_address"] = bill
	}
	if len(in.NoteAttributes) > 0 {
		body["note_attributes"] = in.NoteAttributes
	}
	var out ShopifyOrder
	if err := c.doJSON(ctx, http.MethodPost, "/api/orders", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListFulfillments calls GET /api/orders/{id}/fulfillments.
func (c *ShopifyClient) ListFulfillments(ctx context.Context, govelShopID string, orderID int64) ([]ShopifyFulfillment, error) {
	q := url.Values{}
	q.Set("govel_shop_id", govelShopID)
	path := "/api/orders/" + strconv.FormatInt(orderID, 10) + "/fulfillments"
	var payload struct {
		Fulfillments []ShopifyFulfillment `json:"fulfillments"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, q, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Fulfillments, nil
}

// CreateFulfillment calls POST /api/orders/{id}/fulfillments (orders.fulfillment_write).
func (c *ShopifyClient) CreateFulfillment(ctx context.Context, govelShopID string, orderID int64, in CreateFulfillmentInput) (*ShopifyFulfillment, error) {
	body := map[string]any{
		"govel_shop_id":     govelShopID,
		"notify_customer":   in.NotifyCustomer,
		"tracking_company":  in.TrackingCompany,
		"tracking_number":   in.TrackingNumber,
		"tracking_url":      in.TrackingURL,
	}
	path := "/api/orders/" + strconv.FormatInt(orderID, 10) + "/fulfillments"
	var out ShopifyFulfillment
	if err := c.doJSON(ctx, http.MethodPost, path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFulfillmentTracking calls POST /api/fulfillments/{id}/tracking.
func (c *ShopifyClient) UpdateFulfillmentTracking(ctx context.Context, govelShopID string, fulfillmentID int64, in CreateFulfillmentInput) (*ShopifyFulfillment, error) {
	body := map[string]any{
		"govel_shop_id":    govelShopID,
		"notify_customer":  in.NotifyCustomer,
		"tracking_company": in.TrackingCompany,
		"tracking_number":  in.TrackingNumber,
		"tracking_url":     in.TrackingURL,
	}
	path := "/api/fulfillments/" + strconv.FormatInt(fulfillmentID, 10) + "/tracking"
	var out ShopifyFulfillment
	if err := c.doJSON(ctx, http.MethodPost, path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
