package defs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	bnet "github.com/d2orbc/bungie-api-go"
)

func NewCache(api *bnet.API) *Cache {
	return &Cache{api: api, m: make(map[string]*cachedTable)}
}

type Cache struct {
	api *bnet.API

	mu       sync.Mutex
	manifest bnet.Manifest
	m        map[string]*cachedTable
}

type cachedTable struct {
	mu      sync.Mutex
	version string
	defs    map[uint32]json.RawMessage
}

func (c *Cache) GetDef(table string, hash uint32, out any) error {
	ctx := context.Background()
	if err := c.ensureTable(ctx, table); err != nil {
		return err
	}
	c.mu.Lock()
	t := c.m[table]
	c.mu.Unlock()
	t.mu.Lock()
	def, ok := t.defs[hash]
	t.mu.Unlock()
	if !ok {
		c.CheckUpdates(ctx)
		t.mu.Lock()
		def, ok = t.defs[hash]
		t.mu.Unlock()
	}
	if !ok {
		return fmt.Errorf("missing entry")
	}
	return json.Unmarshal(def, out)
}

func (c *Cache) ensureTable(ctx context.Context, table string) error {
	if err := c.ensureManifest(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	if c.m[table] == nil {
		c.m[table] = &cachedTable{}
	}
	t := c.m[table]
	t.mu.Lock()
	defer t.mu.Unlock()
	mani := c.manifest
	c.mu.Unlock()

	if t.version == mani.Version {
		return nil
	}
	path, ok := mani.JsonWorldComponentContentPaths["en"][table]
	if !ok {
		return fmt.Errorf("unknown definition table %q", table)
	}
	resp, err := http.Get("https://www.bungie.net/" + path)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		return errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var newDefs map[uint32]json.RawMessage
	if err := json.Unmarshal(body, &newDefs); err != nil {
		return err
	}
	t.defs = newDefs
	t.version = mani.Version
	return nil
}

func (c *Cache) CheckUpdates(ctx context.Context) error {
	manifest, err := c.api.Destiny2GetDestinyManifest(ctx, bnet.Destiny2GetDestinyManifestRequest{})
	if err != nil {
		return err
	}
	if manifest.Response.Version == "" {
		return errors.New("missing manifest")
	}
	c.mu.Lock()
	c.manifest = manifest.Response
	c.mu.Unlock()
	return nil
}

// ensureManifest ensures that a manifest is loaded.
func (c *Cache) ensureManifest(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.manifest.Version != "" {
		return nil
	}
	manifest, err := c.api.Destiny2GetDestinyManifest(ctx, bnet.Destiny2GetDestinyManifestRequest{})
	if err != nil {
		return err
	}
	if manifest.Response.Version == "" {
		return errors.New("missing manifest")
	}
	c.manifest = manifest.Response
	return nil
}
