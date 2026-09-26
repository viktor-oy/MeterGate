package cache

import (
	"context"
	"sync"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/telemetry"
	"golang.org/x/sync/singleflight"
)

const (
	numShards = 32
	offset32  = 2166136261
	prime32   = 16777619
)

// TenantData holds the tenant and plan metadata resolved from an API Key.
type TenantData struct {
	TenantID  string
	Plan      string
	ExpiresAt time.Time
}

// APIKeyHash represents the raw 32-byte SHA-256 hash of an API key, used as a zero-allocation map key.
type APIKeyHash [32]byte

// cacheShard is a single shard of the concurrent map, protected by an RWMutex.
// Padding prevents false sharing between cache lines on multi-core systems.
type cacheShard struct {
	sync.RWMutex
	data map[APIKeyHash]TenantData
	_    [64]byte // CPU cache line padding
}

// L1Cache implements a highly concurrent, zero-allocation memory cache.
type L1Cache struct {
	shards []*cacheShard
	sfg    singleflight.Group
	ttl    int64
}

// NewL1Cache initializes the L1 cache.
func NewL1Cache(ttlSeconds int) *L1Cache {
	c := &L1Cache{
		shards: make([]*cacheShard, numShards),
		ttl:    int64(ttlSeconds),
	}
	for i := 0; i < numShards; i++ {
		c.shards[i] = &cacheShard{
			data: make(map[APIKeyHash]TenantData),
		}
	}
	return c
}

// hashBytesFNV1a efficiently hashes a byte slice to an index
func hashBytesFNV1a(data []byte) uint32 {
	var hash uint32 = offset32
	for _, b := range data {
		hash ^= uint32(b)
		hash *= prime32
	}
	return hash
}

// getShard resolves which shard protects the given key
func (c *L1Cache) getShard(keyHash APIKeyHash) *cacheShard {
	return c.shards[hashBytesFNV1a(keyHash[:])%numShards]
}

// GetTenantData retrieves tenant data from the cache.
// Returns false if not found.
func (c *L1Cache) GetTenantData(keyHash APIKeyHash) (TenantData, bool) {
	shard := c.getShard(keyHash)
	shard.RLock()
	data, found := shard.data[keyHash]
	shard.RUnlock()

	// Enforce TTL lazily without locking for writes. time.Now().After() uses the monotonic clock.
	if found && time.Now().After(data.ExpiresAt) {
		return TenantData{}, false
	}
	return data, found
}

// SetTenantData sets tenant data into the cache with the configured TTL.
func (c *L1Cache) SetTenantData(keyHash APIKeyHash, data TenantData) {
	data.ExpiresAt = time.Now().Add(time.Duration(c.ttl) * time.Second)
	shard := c.getShard(keyHash)
	shard.Lock()
	shard.data[keyHash] = data
	shard.Unlock()
}



// FetchTenantWithSingleflight deduplicates simultaneous requests for the same API key,
// preventing a Thundering Herd effect on the Redis L2 or PostgreSQL database.
func (c *L1Cache) FetchTenantWithSingleflight(keyHash APIKeyHash, fetchFn func() (TenantData, error)) (TenantData, error) {
	// First check cache normally (lock-free on the group, but RWMutex on the shard)
	// FAST PATH: Zero-allocation cache hit!
	if data, found := c.GetTenantData(keyHash); found {
		telemetry.CacheOperations.WithLabelValues("L1", "hit").Inc()
		return data, nil
	}

	// SLOW PATH: Cache miss. We allocate a string for singleflight and Redis network boundary.
	sfKey := string(keyHash[:])

	// Not found, use singleflight to collapse identical concurrent fetchFn calls
	res, err, _ := c.sfg.Do(sfKey, func() (interface{}, error) {
		// Double check inside singleflight in case another flight just finished and populated the cache
		if data, found := c.GetTenantData(keyHash); found {
			telemetry.CacheOperations.WithLabelValues("L1", "hit").Inc()
			return data, nil
		}
		
		telemetry.CacheOperations.WithLabelValues("L1", "miss").Inc()
		// Actually fetch from L2/DB
		data, fetchErr := fetchFn()
		if fetchErr != nil {
			return TenantData{}, fetchErr
		}
		
		// Update L1 cache with dynamic TTL
		c.SetTenantData(keyHash, data)
		return data, nil
	})

	if err != nil {
		return TenantData{}, err
	}
	return res.(TenantData), nil
}

// StartSweeper begins a background goroutine that passively cleans up expired keys 
// from the shards every 10 seconds to prevent memory leaks.
func (c *L1Cache) StartSweeper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				for _, shard := range c.shards {
					shard.Lock()
					for k, v := range shard.data {
						if now.After(v.ExpiresAt) {
							delete(shard.data, k)
						}
					}
					shard.Unlock()
				}
			}
		}
	}()
}
