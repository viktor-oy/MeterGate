//go:build integration

package policy

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRateLimiter_Integration_Accumulate(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("Failed to parse REDIS_URL: %v", err)
	}

	client := redis.NewClient(opts)
	ctx := context.Background()

	// Clear out any existing state for tests
	client.FlushDB(ctx)

	// Create RateLimiter
	rl := NewRateLimiter(client)
	
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 1. Start accumulator with known tenants and routes
	shardCount := 3
	rl.Track("tenant-int", "route-int", shardCount, 60)

	rl.StartBackgroundAggregator(ctx)

	// 2. Simulate raw traffic into the Redis salts (shard_0, shard_1, shard_2)
	now := time.Now().Unix()
	window := now / 60

	k0 := "metergate:rate:tenant-int:route-int:" + strconv.FormatInt(window, 10) + ":0"
	k1 := "metergate:rate:tenant-int:route-int:" + strconv.FormatInt(window, 10) + ":1"
	k2 := "metergate:rate:tenant-int:route-int:" + strconv.FormatInt(window, 10) + ":2"

	client.IncrBy(ctx, k0, 5)
	client.IncrBy(ctx, k1, 10)
	client.IncrBy(ctx, k2, 15)

	// Wait for accumulator to sweep
	time.Sleep(200 * time.Millisecond)

	// 3. Verify L1 cache has the accurate sum (5 + 10 + 15 = 30)
	key := TargetKey{TenantID: "tenant-int", RouteID: "route-int"}
	sumShard := rl.sumShards[rl.getShardIndex("tenant-int", "route-int")]
	
	sumShard.RLock()
	sum, exists := sumShard.data[key]
	sumShard.RUnlock()

	if !exists {
		t.Fatalf("Expected sumShards to contain tenant-int/route-int, but it was missing")
	}

	if sum != 30 {
		t.Errorf("Expected sum to be 30, got %d", sum)
	}
}
