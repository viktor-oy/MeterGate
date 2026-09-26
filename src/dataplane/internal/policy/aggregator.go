package policy

import (
	"context"
	"strconv"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/telemetry"
)

// StartBackgroundAggregator spins up the async worker that calculates SUM(shard_0..shard_9)
// and updates the local L1 cache every 100ms.
// Note: This file contains acceptable background allocations (slices for MGET)
// and is explicitly excluded from the zero-allocation hot path check.
func (rl *RateLimiter) StartBackgroundAggregator(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Iterate through all shards of tracked active targets
				for i := 0; i < len(rl.targetShards); i++ {
					tShard := rl.targetShards[i]
					tShard.RLock()
					// Copy keys/values to avoid holding the lock during Redis network I/O
					targets := make([]ActiveTarget, 0, len(tShard.data))
					for _, val := range tShard.data {
						targets = append(targets, val)
					}
					tShard.RUnlock()

					for _, target := range targets {
						window := time.Now().Unix() / target.Divisor
						var sum int64
						
						// If shardCount is 1, it's just a standard INCR key
						// If shardCount > 1, we MGET all salt buckets
						if target.ShardCount == 1 {
							k, bufPtr := FormatRateLimitKey(target.TenantID, target.RouteID, 0, window)
							
							startRedis := time.Now()
							val, _ := rl.client.Get(ctx, k).Int64()
							telemetry.RedisOperationDuration.WithLabelValues("GET").Observe(time.Since(startRedis).Seconds())
							
							sum = val
							
							// PERF: Since bufPtr is already a heap pointer from the pool, passing it to 
							// sync.Pool.Put(interface{}) does not allocate a new interface wrapper on the heap.
							redisKeyPool.Put(bufPtr)
						} else {
							// This allocation is acceptable on the background ticker path
							keys := make([]string, target.ShardCount)
							bufPtrs := make([]*[]byte, target.ShardCount)
							for j := 0; j < target.ShardCount; j++ {
								k, bufPtr := FormatRateLimitKey(target.TenantID, target.RouteID, j, window)
								keys[j] = k
								bufPtrs[j] = bufPtr
							}
							
							// Pipeline or MGET
							startRedis := time.Now()
							vals, _ := rl.client.MGet(ctx, keys...).Result()
							telemetry.RedisOperationDuration.WithLabelValues("MGET").Observe(time.Since(startRedis).Seconds())
							
							for _, v := range vals {
								if v != nil {
									if str, ok := v.(string); ok {
										parsed, _ := strconv.ParseInt(str, 10, 64)
										sum += parsed
									}
								}
							}
							
							for _, ptr := range bufPtrs {
								// PERF: Since ptr is already a heap pointer from the pool, passing it to 
								// sync.Pool.Put(interface{}) does not allocate a new interface wrapper on the heap.
								redisKeyPool.Put(ptr)
							}
						}
						
						// Update L1
						key := TargetKey{TenantID: target.TenantID, RouteID: target.RouteID}
						sShard := rl.sumShards[rl.getShardIndex(target.TenantID, target.RouteID)]
						sShard.Lock()
						sShard.data[key] = int(sum)
						sShard.Unlock()
					}
				}
			}
		}
	}()
}
