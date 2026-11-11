package syncio

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Connection is a Syncio multi-shop sync link (source ↔ destinations).
type Connection struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	SourceShopID          string          `json:"source_shop_id"`
	DestinationShopIDs    []string        `json:"destination_shop_ids"`
	Enabled               bool            `json:"enabled"`
	InventorySyncEnabled  bool            `json:"inventory_sync_enabled"`
	ProductSyncEnabled    bool            `json:"product_sync_enabled"`
	InventoryDirection    string          `json:"inventory_direction"` // source_to_dest | dest_to_source | two_way
	MatchByBarcode        bool            `json:"match_by_barcode"`
	CreateMissingProducts bool            `json:"create_missing_products"`
	AttributeToggles      map[string]bool `json:"attribute_toggles"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// ProductMapping links a source variant to a destination variant by SKU/barcode.
type ProductMapping struct {
	ID                string     `json:"id"`
	ConnectionID      string     `json:"connection_id"`
	DestinationShopID string     `json:"destination_shop_id"`
	MatchKey          string     `json:"match_key"`
	MatchKind         string     `json:"match_kind"` // sku | barcode
	SourceProductID   string     `json:"source_product_id,omitempty"`
	SourceVariantID   string     `json:"source_variant_id,omitempty"`
	DestProductID     string     `json:"dest_product_id,omitempty"`
	DestVariantID     string     `json:"dest_variant_id,omitempty"`
	LastSyncedAt      *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type storeFile struct {
	Connections []Connection     `json:"connections"`
	Mappings    []ProductMapping `json:"mappings"`
}

// Store persists connections and mappings to a JSON file (host Postgres via manifest at install).
type Store struct {
	mu   sync.RWMutex
	path string
	data storeFile
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, data: storeFile{}}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, s.flushLocked()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}
	return s, nil
}

func (s *Store) flushLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) BootstrapFromJSON(raw string) error {
	if raw == "" {
		return nil
	}
	var items []Connection
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return fmt.Errorf("SYNCIO_CONNECTIONS_JSON: %w", err)
	}
	for _, c := range items {
		if _, err := s.Upsert(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) List() []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Connection, len(s.data.Connections))
	copy(out, s.data.Connections)
	return out
}

func (s *Store) Get(id string) (Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.data.Connections {
		if c.ID == id {
			return c, true
		}
	}
	return Connection{}, false
}

func normalizeConnection(c Connection, isCreate bool) Connection {
	if c.InventoryDirection == "" {
		c.InventoryDirection = "two_way"
	}
	switch c.InventoryDirection {
	case "source_to_dest", "dest_to_source", "two_way":
	default:
		c.InventoryDirection = "two_way"
	}
	if isCreate {
		// JSON omitempty / zero values: enable sensible defaults on first insert.
		if !c.MatchByBarcode && c.ID == "" {
			c.MatchByBarcode = true
		}
	}
	if c.AttributeToggles == nil {
		c.AttributeToggles = DefaultAttributeToggles()
	} else {
		defs := DefaultAttributeToggles()
		for k, v := range defs {
			if _, ok := c.AttributeToggles[k]; !ok {
				c.AttributeToggles[k] = v
			}
		}
	}
	if c.DestinationShopIDs == nil {
		c.DestinationShopIDs = []string{}
	}
	return c
}

func (s *Store) Upsert(in Connection) (Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	isCreate := in.ID == ""
	in = normalizeConnection(in, isCreate)
	if in.Name == "" || in.SourceShopID == "" {
		return Connection{}, fmt.Errorf("name and source_shop_id are required")
	}
	if isCreate {
		in.ID = newID()
		in.CreatedAt = now
		in.UpdatedAt = now
		s.data.Connections = append(s.data.Connections, in)
		return in, s.flushLocked()
	}
	for i, c := range s.data.Connections {
		if c.ID == in.ID {
			in.CreatedAt = c.CreatedAt
			in.UpdatedAt = now
			s.data.Connections[i] = in
			return in, s.flushLocked()
		}
	}
	in.CreatedAt = now
	in.UpdatedAt = now
	s.data.Connections = append(s.data.Connections, in)
	return in, s.flushLocked()
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	kept := s.data.Connections[:0]
	for _, c := range s.data.Connections {
		if c.ID == id {
			found = true
			continue
		}
		kept = append(kept, c)
	}
	s.data.Connections = kept
	if found {
		maps := s.data.Mappings[:0]
		for _, m := range s.data.Mappings {
			if m.ConnectionID == id {
				continue
			}
			maps = append(maps, m)
		}
		s.data.Mappings = maps
		_ = s.flushLocked()
	}
	return found
}

func (s *Store) ListMappings(connectionID, destShopID string) []ProductMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ProductMapping
	for _, m := range s.data.Mappings {
		if connectionID != "" && m.ConnectionID != connectionID {
			continue
		}
		if destShopID != "" && m.DestinationShopID != destShopID {
			continue
		}
		out = append(out, m)
	}
	return out
}

func (s *Store) UpsertMapping(in ProductMapping) (ProductMapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if in.ConnectionID == "" || in.DestinationShopID == "" || in.MatchKey == "" {
		return ProductMapping{}, fmt.Errorf("connection_id, destination_shop_id, match_key required")
	}
	if in.MatchKind == "" {
		in.MatchKind = "sku"
	}
	for i, m := range s.data.Mappings {
		if m.ConnectionID == in.ConnectionID && m.DestinationShopID == in.DestinationShopID && m.MatchKey == in.MatchKey {
			in.ID = m.ID
			in.CreatedAt = m.CreatedAt
			in.UpdatedAt = now
			s.data.Mappings[i] = in
			return in, s.flushLocked()
		}
	}
	if in.ID == "" {
		in.ID = newID()
	}
	in.CreatedAt = now
	in.UpdatedAt = now
	s.data.Mappings = append(s.data.Mappings, in)
	return in, s.flushLocked()
}
