package cache

import (
	"sync"
	"sync/atomic"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
	"golang.org/x/sync/singleflight"
)

const (
	numShards = 32
	offset32  = 2166136261
	prime32   = 16777619
)

// TenantData holds the tenant and plan metadata resolved from an API Key.
type TenantData struct {
	TenantID string
	Plan     string
}

// cacheShard is a single shard of the concurrent map, protected by an RWMutex.
type cacheShard struct {
	sync.RWMutex
	data map[string]TenantData
}

// L1Cache implements a highly concurrent, zero-allocation memory cache.
type L1Cache struct {
	shards []*cacheShard
	routes atomic.Pointer[[]config.RouteConfig]
	sfg    singleflight.Group
}

// NewL1Cache initializes the L1 cache.
func NewL1Cache() *L1Cache {
	c := &L1Cache{
		shards: make([]*cacheShard, numShards),
	}
	for i := 0; i < numShards; i++ {
		c.shards[i] = &cacheShard{
			data: make(map[string]TenantData),
		}
	}
	// Initialize empty routes
	emptyRoutes := make([]config.RouteConfig, 0)
	c.routes.Store(&emptyRoutes)
	return c
}

// hashStringFNV1a computes the FNV-1a hash of a string natively without allocating a byte slice.
// This is critical for zero-allocation on the hot path.
func hashStringFNV1a(key string) uint32 {
	hash := uint32(offset32)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= prime32
	}
	return hash
}

// getShard returns the specific lock shard for the given API key.
func (c *L1Cache) getShard(key string) *cacheShard {
	return c.shards[hashStringFNV1a(key)%numShards]
}

// GetTenantData retrieves tenant data from the cache.
// Returns false if not found.
func (c *L1Cache) GetTenantData(apiKey string) (TenantData, bool) {
	shard := c.getShard(apiKey)
	shard.RLock()
	data, found := shard.data[apiKey]
	shard.RUnlock()
	return data, found
}

// SetTenantData sets tenant data into the cache.
func (c *L1Cache) SetTenantData(apiKey string, data TenantData) {
	shard := c.getShard(apiKey)
	shard.Lock()
	shard.data[apiKey] = data
	shard.Unlock()
}

// LoadRoutes returns a pointer to the current route table safely.
// This uses the RCU (Read-Copy-Update) pattern via atomic.Pointer.
func (c *L1Cache) LoadRoutes() *[]config.RouteConfig {
	return c.routes.Load()
}

// UpdateRoutes applies the Read-Copy-Update pattern to swap the route table entirely lock-free.
func (c *L1Cache) UpdateRoutes(newRoutes []config.RouteConfig) {
	c.routes.Store(&newRoutes)
}

// FetchTenantWithSingleflight deduplicates simultaneous requests for the same API key,
// preventing a Thundering Herd effect on the Redis L2 or PostgreSQL database.
func (c *L1Cache) FetchTenantWithSingleflight(apiKey string, fetchFn func() (TenantData, error)) (TenantData, error) {
	// First check cache normally (lock-free on the group, but RWMutex on the shard)
	if data, found := c.GetTenantData(apiKey); found {
		return data, nil
	}

	// Not found, use singleflight to collapse identical concurrent fetchFn calls
	res, err, _ := c.sfg.Do(apiKey, func() (interface{}, error) {
		// Double check inside singleflight in case another flight just finished and populated the cache
		if data, found := c.GetTenantData(apiKey); found {
			return data, nil
		}
		
		// Actually fetch from L2/DB
		data, fetchErr := fetchFn()
		if fetchErr != nil {
			return TenantData{}, fetchErr
		}
		
		// Update L1 cache
		c.SetTenantData(apiKey, data)
		return data, nil
	})

	if err != nil {
		return TenantData{}, err
	}
	return res.(TenantData), nil
}
