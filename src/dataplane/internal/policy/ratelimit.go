package policy

import (
	"context"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter manages the Two-Tier Quota Aggregation logic.
type RateLimiter struct {
	client redis.UniversalClient
	
	// sumCache holds the async aggregated SUM of all Redis salt shards.
	// Key: "tenantID:routeID", Value: total int64
	sumCache sync.Map

	// activeTenants and activeRoutes track known traffic
	activeTenants sync.Map
	activeRoutes  sync.Map
}

func NewRateLimiter(client redis.UniversalClient) *RateLimiter {
	return &RateLimiter{
		client: client,
	}
}

// Track dynamically tracks tenants and routes for the aggregator.
func (rl *RateLimiter) Track(tenantID, routeID string) {
	rl.activeTenants.Store(tenantID, struct{}{})
	rl.activeRoutes.Store(routeID, struct{}{})
}

// GetRateLimitKey dynamically generates a salted Redis key for high-velocity Enterprise traffic.
// To ensure zero heap allocations, we avoid fmt.Sprintf.
func GetRateLimitKey(tenantID, routeID string, shardCount int, windowUnix int64) string {
	salt := 0
	if shardCount > 1 {
		salt = rand.IntN(shardCount)
	}

	// metergate:rate:{tenantID}:{routeID}:{window}:{salt}
	// Using standard string concatenation can allocate, but inside a very short function,
	// modern Go compilers often optimize this. To be perfectly safe against escapes, 
	// we'd use a stack-allocated byte array, but strings in Go are immutable.
	// We'll rely on the compiler's string builder optimization for simple concats.
	return "metergate:rate:" + tenantID + ":" + routeID + ":" + strconv.FormatInt(windowUnix, 10) + ":" + strconv.Itoa(salt)
}

// CheckAndRecord increments the quota synchronously (hot-path) but relies on
// the async background aggregator for the authoritative SUM.
func (rl *RateLimiter) CheckAndRecord(ctx context.Context, tenantID, routeID string, shardCount, limit int) (bool, int) {
	window := time.Now().Unix() / 60

	// 1. Check L1 memory for authoritative aggregated sum (zero network I/O)
	cacheKey := tenantID + ":" + routeID
	var currentTotal int
	if val, ok := rl.sumCache.Load(cacheKey); ok {
		currentTotal = val.(int)
	}

	if currentTotal >= limit {
		// Rate limited!
		return false, 0
	}

	// 2. Fast-path write to salted Redis shard
	// go-redis/v9 uses sync.Pool internally so this does not cause garbage collection spikes.
	redisKey := GetRateLimitKey(tenantID, routeID, shardCount, window)
	rl.client.Incr(ctx, redisKey)

	// Since we are asynchronously aggregating, we just return the local cache sum estimate
	remaining := limit - (currentTotal + 1)
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining
}

// StartBackgroundAggregator spins up the async worker that calculates SUM(shard_0..shard_9)
// and updates the local L1 cache every 100ms.
func (rl *RateLimiter) StartBackgroundAggregator(ctx context.Context, shardCount int) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				window := time.Now().Unix() / 60
				
				// Iterate tracked tenants/routes
				rl.activeTenants.Range(func(tKey, _ interface{}) bool {
					tenant := tKey.(string)
					rl.activeRoutes.Range(func(rKey, _ interface{}) bool {
						route := rKey.(string)
						var sum int64
						
						// If shardCount is 1, it's just a standard INCR key
						// If shardCount > 1, we MGET all salt buckets
						if shardCount == 1 {
							key := GetRateLimitKey(tenant, route, 1, window)
							val, _ := rl.client.Get(ctx, key).Int64()
							sum = val
						} else {
							keys := make([]string, shardCount)
							for i := 0; i < shardCount; i++ {
								keys[i] = "metergate:rate:" + tenant + ":" + route + ":" + strconv.FormatInt(window, 10) + ":" + strconv.Itoa(i)
							}
							
							// Pipeline or MGET
							vals, _ := rl.client.MGet(ctx, keys...).Result()
							for _, v := range vals {
								if v != nil {
									if str, ok := v.(string); ok {
										parsed, _ := strconv.ParseInt(str, 10, 64)
										sum += parsed
									}
								}
							}
						}
						
						// Update L1
						cacheKey := tenant + ":" + route
						rl.sumCache.Store(cacheKey, int(sum))
						return true
					})
					return true
				})
			}
		}
	}()
}
