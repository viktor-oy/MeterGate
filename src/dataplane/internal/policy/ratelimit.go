package policy

import (
	"context"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/telemetry"
	"github.com/redis/go-redis/v9"
)

// RateLimiter manages the Two-Tier Quota Aggregation logic.
type RateLimiter struct {
	client redis.UniversalClient

	// sumShards holds the async aggregated SUM of all Redis salt shards.
	sumShards [32]*sumShard

	// targetShards tracks the dynamic ShardCount and Divisor for each tenant-route pair
	targetShards [32]*targetShard
}

type TargetKey struct {
	TenantID string
	RouteID  string
}

type targetShard struct {
	sync.RWMutex
	data map[TargetKey]ActiveTarget
	_    [64]byte
}

type sumShard struct {
	sync.RWMutex
	data map[TargetKey]int
	_    [64]byte
}

type ActiveTarget struct {
	TenantID   string
	RouteID    string
	ShardCount int
	Divisor    int64
}

func NewRateLimiter(client redis.UniversalClient) *RateLimiter {
	rl := &RateLimiter{
		client: client,
	}
	for i := 0; i < len(rl.targetShards); i++ {
		rl.targetShards[i] = &targetShard{
			data: make(map[TargetKey]ActiveTarget),
		}
		rl.sumShards[i] = &sumShard{
			data: make(map[TargetKey]int),
		}
	}
	return rl
}

func (rl *RateLimiter) getShardIndex(tenantID, routeID string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(tenantID); i++ {
		hash ^= uint32(tenantID[i])
		hash *= 16777619
	}
	for i := 0; i < len(routeID); i++ {
		hash ^= uint32(routeID[i])
		hash *= 16777619
	}
	return hash % uint32(len(rl.targetShards))
}

// Track dynamically tracks targets for the aggregator.
func (rl *RateLimiter) Track(tenantID, routeID string, shardCount int, divisor int64) {
	key := TargetKey{TenantID: tenantID, RouteID: routeID}
	shard := rl.targetShards[rl.getShardIndex(tenantID, routeID)]

	// FAST PATH (Lock-Free Read): Double-Checked Locking implementation to completely avoid
	// Mutex contention and interface boxing allocations on the hot path.
	shard.RLock()
	_, exists := shard.data[key]
	shard.RUnlock()

	if exists {
		// PERF OPTIMIZATION: Key exists! Exit instantly. No locks, no heap allocations.
		return
	}

	// SLOW PATH (Write Lock): Only hit on the very first request for a new tenant route.
	shard.Lock()
	// Double-check just in case another goroutine wrote it while we escalated locks
	if _, exists := shard.data[key]; !exists {
		shard.data[key] = ActiveTarget{
			TenantID:   tenantID,
			RouteID:    routeID,
			ShardCount: shardCount,
			Divisor:    divisor,
		}
	}
	shard.Unlock()
}

var redisKeyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 128)
		return &b
	},
}

// FormatRateLimitKey dynamically generates a zero-allocation Redis key.
// EXPLICIT TRUST WARNING: We use unsafe.String to pass this byte slice to go-redis zero-copy.
// This relies entirely on the assumption that go-redis synchronously serializes this string to the
// network socket and discards it. If go-redis internally caches or retains the string key, this
// pooled buffer will cause memory corruption!
//
// PERF: We deliberately return the *[]byte pointer directly from the pool.
// When passed back to sync.Pool.Put(interface{}), Go recognizes it is already a heap pointer
// and completely avoids allocating a new interface wrapper on the heap.
func FormatRateLimitKey(tenantID, routeID string, salt int, windowUnix int64) (string, *[]byte) {
	bufPtr := redisKeyPool.Get().(*[]byte)
	buf := *bufPtr
	buf = buf[:0] // Reset length

	buf = append(buf, "metergate:rate:"...)
	buf = append(buf, tenantID...)
	buf = append(buf, ':')
	buf = append(buf, routeID...)
	buf = append(buf, ':')
	buf = strconv.AppendInt(buf, windowUnix, 10)
	buf = append(buf, ':')
	buf = strconv.AppendInt(buf, int64(salt), 10)

	// Zero-allocation cast
	str := unsafe.String(unsafe.SliceData(buf), len(buf))
	return str, bufPtr
}

// GetRateLimitKey wraps FormatRateLimitKey with salt distribution.
// To ensure zero heap allocations, we avoid fmt.Sprintf.
// (See FormatRateLimitKey for why *[]byte is returned for zero-allocation pool returns).
func GetRateLimitKey(tenantID, routeID string, shardCount int, windowUnix int64) (string, *[]byte) {
	salt := 0
	if shardCount > 1 {
		salt = rand.IntN(shardCount)
	}

	return FormatRateLimitKey(tenantID, routeID, salt, windowUnix)
}

// CheckAndRecord increments the quota synchronously (hot-path) but relies on
// the async background aggregator for the authoritative SUM.
func (rl *RateLimiter) CheckAndRecord(ctx context.Context, tenantID, routeID string, shardCount, limit int, divisor int64) (bool, int) {
	window := time.Now().Unix() / divisor

	// 1. Check L1 memory for authoritative aggregated sum (zero network I/O)
	// We use the sharded map to avoid string concat and interface boxing allocations.
	key := TargetKey{TenantID: tenantID, RouteID: routeID}
	sumShard := rl.sumShards[rl.getShardIndex(tenantID, routeID)]

	sumShard.RLock()
	currentTotal := sumShard.data[key]
	sumShard.RUnlock()

	if currentTotal >= limit {
		// Rate limited!
		return false, 0
	}

	// 2. Fast-path write to salted Redis shard
	// We receive the pooled buffer ptr and MUST manually return it after go-redis serializes it.
	redisKey, bufPtr := GetRateLimitKey(tenantID, routeID, shardCount, window)

	startRedis := time.Now()
	rl.client.Incr(ctx, redisKey)
	telemetry.RedisOperationDuration.WithLabelValues("INCR").Observe(time.Since(startRedis).Seconds())

	// Critical: Return the buffer to the pool after the synchronous Redis call.
	// Since bufPtr is already a heap pointer, sync.Pool.Put(interface{}) will not allocate memory
	// for the interface wrapper.
	redisKeyPool.Put(bufPtr)

	// Since we are asynchronously aggregating, we just return the local cache sum estimate
	remaining := limit - (currentTotal + 1)
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining
}
