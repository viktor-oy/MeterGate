package policy_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/cache"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/policy"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// mockRedisClient is a lightweight stub that implements just enough of redis.UniversalClient
// to satisfy the unit tests without needing a real Redis instance.
type mockRedisClient struct {
	redis.UniversalClient
	mockData map[string]string
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	if val, ok := m.mockData[key]; ok {
		return redis.NewStringResult(val, nil)
	}
	return redis.NewStringResult("", redis.Nil)
}

func (m *mockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	return redis.NewIntResult(1, nil)
}

func TestCoreEngine_Evaluate_Unit(t *testing.T) {
	cfg := &config.MeterGateConfig{
		ProtectedRoutes: []config.RouteConfig{
			{
				ID:         "graphql",
				Method:     "POST",
				PathPrefix: "/graphql",
				PlanGroup:  "api",
			},
			{
				ID:         "metrics",
				Method:     "GET",
				PathPrefix: "/metrics",
				PlanGroup:  "internal",
			},
		},
		Plans: map[string]config.Plan{
			"standard": {
				ShardCount: 1,
				Groups: map[string]config.RateLimitGroupConfig{
					"api":      {Limit: 100, Unit: "minute"},
					"internal": {Limit: 100, Unit: "minute"},
				},
			},
		},
	}

	l1 := cache.NewL1Cache()
	mockRedis := &mockRedisClient{
		mockData: make(map[string]string),
	}
	rl := policy.NewRateLimiter(mockRedis)
	engine := policy.NewCoreEngine(cfg, l1, rl, mockRedis)
	ctx := context.Background()

	// 1. Pre-seed the mock L2 Redis cache
	apiKey := "valid-key"
	secret := "local-demo-secret-change-me"
	h := sha256.New()
	h.Write([]byte(secret + ":" + apiKey))
	keyHash := hex.EncodeToString(h.Sum(nil))

	tenantData := map[string]interface{}{
		"tenantId": "tenant-123",
		"planCode": "standard",
		"revoked":  false,
	}
	val, _ := json.Marshal(tenantData)
	mockRedis.mockData["metergate:cache:apikey:"+keyHash] = string(val)

	t.Run("Missing API Key", func(t *testing.T) {
		evalCtx := policy.EvaluationContext{
			APIKey: "",
			Method: "POST",
			Path:   "/graphql",
		}

		decision := engine.Evaluate(ctx, evalCtx)
		assert.False(t, decision.Allowed)
		assert.Equal(t, policy.ReasonMissingHeader, decision.Reason)
	})

	t.Run("Invalid API Key (Not in Mock Redis)", func(t *testing.T) {
		evalCtx := policy.EvaluationContext{
			APIKey: "invalid-key",
			Method: "POST",
			Path:   "/graphql",
		}

		decision := engine.Evaluate(ctx, evalCtx)
		assert.False(t, decision.Allowed)
		assert.Equal(t, policy.ReasonInvalidKey, decision.Reason)
	})

	t.Run("Route Not Found", func(t *testing.T) {
		evalCtx := policy.EvaluationContext{
			APIKey: "valid-key",
			Method: "POST",
			Path:   "/unknown",
		}

		decision := engine.Evaluate(ctx, evalCtx)
		assert.False(t, decision.Allowed)
		assert.Equal(t, policy.ReasonRouteNotFound, decision.Reason)
	})

	t.Run("Valid Route - Allowed", func(t *testing.T) {
		evalCtx := policy.EvaluationContext{
			APIKey: "valid-key",
			Method: "GET",
			Path:   "/metrics/prometheus",
		}

		decision := engine.Evaluate(ctx, evalCtx)
		assert.True(t, decision.Allowed)
		assert.Equal(t, policy.ReasonAllowed, decision.Reason)
		assert.Equal(t, "metrics", decision.RouteID)
		assert.Equal(t, "tenant-123", decision.TenantID)
	})
}

// extendedMockRedisClient adds MGet to the mock for the background aggregator
type extendedMockRedisClient struct {
	*mockRedisClient
}

func (m *extendedMockRedisClient) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	vals := make([]interface{}, len(keys))
	for i, key := range keys {
		if val, ok := m.mockData[key]; ok {
			vals[i] = val
		} else {
			vals[i] = nil
		}
	}
	return redis.NewSliceResult(vals, nil)
}

func (m *extendedMockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	m.mockData[key] = "1" // Simplified tracking for the mock
	return redis.NewIntResult(1, nil)
}

func TestRateLimiter_CheckAndRecord_Unit(t *testing.T) {
	baseMock := &mockRedisClient{
		mockData: make(map[string]string),
	}
	mockRedis := &extendedMockRedisClient{mockRedisClient: baseMock}
	
	rl := policy.NewRateLimiter(mockRedis)
	ctx := context.Background()

	// Before aggregator sweeps, the cache is 0. 
	// Limit is 10, so it should be allowed.
	allowed, remaining := rl.CheckAndRecord(ctx, "tenant-1", "route-1", 1, 10)
	assert.True(t, allowed)
	assert.Equal(t, 9, remaining)

	// Inject sum 10 into mock data for the background aggregator to pick up
	mockRedis.mockData["metergate:rate:tenant-1:route-1:"+time.Now().Format("20060102")] = "10"
	
	rl.Track("tenant-1", "route-1")
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	rl.StartBackgroundAggregator(cancelCtx, 1)
}
