package syncio

import (
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

// ShopifyClient talks to govel-shopify HTTP API (not Shopify Admin directly).
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

type ShopifyVariant struct {
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	SKU               string `json:"sku"`
	Barcode           string `json:"barcode"`
	Price             string `json:"price"`
	InventoryQuantity int32  `json:"inventory_quantity"`
}

type ShopifyProduct struct {
	GovelShopID string           `json:"govel_shop_id"`
	ShopDomain  string           `json:"shop_domain"`
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Status      string           `json:"status"`
	Vendor      string           `json:"vendor"`
	ProductType string           `json:"product_type"`
	Tags        string           `json:"tags"`
	BodyHTML    string           `json:"body_html"`
	Variants    []ShopifyVariant `json:"variants"`
}

func (c *ShopifyClient) getJSON(ctx context.Context, path string, q url.Values, dest any) error {
	if c.BaseURL == "" {
		return fmt.Errorf("SHOPIFY_HTTP_BASE is empty")
	}
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("govel-shopify %s: %s (%s)", path, res.Status, strings.TrimSpace(string(body)))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(body, dest)
}

func (c *ShopifyClient) ListShops(ctx context.Context) ([]ShopifyShop, error) {
	var payload struct {
		Shops []ShopifyShop `json:"shops"`
	}
	if err := c.getJSON(ctx, "/api/shops", nil, &payload); err != nil {
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
	if err := c.getJSON(ctx, "/api/products", q, &payload); err != nil {
		return nil, err
	}
	return payload.Products, nil
}

// SetInventoryQuantity is the intended write path via govel-shopify.
// Until govel-shopify exposes inventory write RPCs, this returns a clear capability error.
func (c *ShopifyClient) SetInventoryQuantity(ctx context.Context, govelShopID, variantID string, qty int32) error {
	if c.BaseURL == "" {
		return fmt.Errorf("SHOPIFY_HTTP_BASE is empty")
	}
	u := c.BaseURL + "/api/inventory/set"
	body := fmt.Sprintf(`{"govel_shop_id":%q,"variant_id":%q,"quantity":%d}`, govelShopID, variantID, qty)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("govel-shopify missing inventory write API (POST /api/inventory/set); needs generic catalog.inventory_write")
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("set inventory: %s (%s)", res.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

// UpdateProductAttributes is the intended product write path via govel-shopify.
func (c *ShopifyClient) UpdateProductAttributes(ctx context.Context, govelShopID, productID string, attrs map[string]any) error {
	if c.BaseURL == "" {
		return fmt.Errorf("SHOPIFY_HTTP_BASE is empty")
	}
	u := c.BaseURL + "/api/products/" + url.PathEscape(productID)
	payload := map[string]any{
		"govel_shop_id": govelShopID,
		"attributes":    attrs,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed {
		return fmt.Errorf("govel-shopify missing product write API (PATCH /api/products/{id}); needs generic catalog.product_write")
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("update product: %s (%s)", res.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}
