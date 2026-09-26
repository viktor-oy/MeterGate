package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/cache"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/telemetry"
	"github.com/redis/go-redis/v9"
)

// Reason string constants for policy decisions
const (
	ReasonAllowed          = "allowed"

	ReasonInvalidKey       = "invalid_api_key"
	ReasonRouteNotFound    = "route_not_found"
	ReasonRateLimited      = "rate_limited"
	ReasonInternalError    = "internal_error"
	ReasonMissingHeader    = "missing_api_key_header"
)

// PolicyDecision is passed by value (zero heap allocation)
type PolicyDecision struct {
	Allowed   bool
	Reason    string
	Remaining int
	Reset     time.Time
	RouteID   string
	Plan      string
	TenantID  string
}

// EvaluationContext encapsulates request metadata for policy evaluation without escaping to the heap
type EvaluationContext struct {
	APIKey   string
	Method   string
	Path     string
	ClientIP string
}

// Engine represents the core policy engine interfaces
type Engine interface {
	Evaluate(ctx context.Context, evalCtx EvaluationContext) PolicyDecision
}

// Ensure the implementation adheres to the interface
var _ Engine = (*CoreEngine)(nil)

type CoreEngine struct {
	cfg         *config.MeterGateConfig
	l1Cache     *cache.L1Cache
	rateLimiter *RateLimiter
	redisClient redis.UniversalClient
	hashSecret  string
}

func NewCoreEngine(cfg *config.MeterGateConfig, l1 *cache.L1Cache, rl *RateLimiter, rc redis.UniversalClient) (*CoreEngine, error) {
	secret := os.Getenv("METERGATE_KEY_HASH_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("METERGATE_KEY_HASH_SECRET environment variable is required")
	}
	return &CoreEngine{
		cfg:         cfg,
		l1Cache:     l1,
		rateLimiter: rl,
		redisClient: rc,
		hashSecret:  secret,
	}, nil
}

// hashAPIKey safely generates the sha256 hash used as the key in L1/L2 Cache.
// PERF OPTIMIZATION: This explicitly returns a stack-allocated [32]byte array (pure value type)
// to be safely used as a strongly-typed, zero-allocation map key in the L1Cache.
func (e *CoreEngine) hashAPIKey(apiKey string) cache.APIKeyHash {
	// Strict overflow validation to guarantee we don't exceed our 256-byte stack buffer.
	totalLen := len(e.hashSecret) + 1 + len(apiKey)
	
	if totalLen > 256 {
		// Fallback to heap allocation for abnormally large keys to prevent stack overflow/panic
		buf := make([]byte, totalLen)
		n := copy(buf, e.hashSecret)
		buf[n] = ':'
		copy(buf[n+1:], apiKey)
		return sha256.Sum256(buf)
	}

	// Hot-path zero-allocation execution
	var buf [256]byte
	n := copy(buf[:], e.hashSecret)
	buf[n] = ':'
	copy(buf[n+1:], apiKey)
	
	return sha256.Sum256(buf[:totalLen])
}

// Evaluate performs the zero-allocation chain of policy checks and instruments telemetry.
func (e *CoreEngine) Evaluate(ctx context.Context, evalCtx EvaluationContext) PolicyDecision {
	start := time.Now()
	decision := e.evaluateInternal(ctx, evalCtx)
	
	telemetry.PolicyEvalDuration.WithLabelValues(decision.RouteID, decision.Plan).Observe(time.Since(start).Seconds())
	decisionStr := "deny"
	if decision.Allowed {
		decisionStr = "allow"
	}
	telemetry.PolicyDecisions.WithLabelValues(decisionStr, decision.Reason, decision.RouteID, decision.Plan).Inc()
	
	return decision
}

func (e *CoreEngine) evaluateInternal(ctx context.Context, evalCtx EvaluationContext) PolicyDecision {
	// 1. Missing API Key Check
	if evalCtx.APIKey == "" {
		return PolicyDecision{Allowed: false, Reason: ReasonMissingHeader}
	}

	keyHash := e.hashAPIKey(evalCtx.APIKey)

	// 1. Check in-memory L1 cache (Read-Only Fast Path)
	tenantData, err := e.l1Cache.FetchTenantWithSingleflight(keyHash, func() (cache.TenantData, error) {
		// The fetch function is only invoked on an L1 Cache Miss.
		
		// Encode to hex string for the Redis network request
		hexKey := hex.EncodeToString(keyHash[:])
		redisKey := "metergate:cache:apikey:" + hexKey
		
		startRedis := time.Now()
		val, err := e.redisClient.Get(ctx, redisKey).Result()
		telemetry.RedisOperationDuration.WithLabelValues("GET").Observe(time.Since(startRedis).Seconds())
		
		if err != nil {
			telemetry.CacheOperations.WithLabelValues("L2", "miss").Inc()
			return cache.TenantData{}, err
		}
		telemetry.CacheOperations.WithLabelValues("L2", "hit").Inc()
		var data struct {
			TenantID string `json:"tenantId"`
			PlanCode string `json:"planCode"`
			Revoked  bool   `json:"revoked"`
		}
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			return cache.TenantData{}, err
		}
		if data.Revoked {
			return cache.TenantData{}, fmt.Errorf("api key revoked")
		}
		return cache.TenantData{
			TenantID: data.TenantID,
			Plan:     data.PlanCode,
		}, nil
	})

	if err != nil || tenantData.TenantID == "" {
		return PolicyDecision{Allowed: false, Reason: ReasonInvalidKey}
	}

	// 3. Route Matching Check
	var matchedRoute *config.RouteConfig
	for i := range e.cfg.ProtectedRoutes {
		route := &e.cfg.ProtectedRoutes[i]
		if route.Method == evalCtx.Method && matchPathPrefix(evalCtx.Path, route.PathPrefix) {
			matchedRoute = route
			break
		}
	}

	if matchedRoute == nil {
		return PolicyDecision{Allowed: false, Reason: ReasonRouteNotFound}
	}

	// 4. Rate Limiting Check
	plan, ok := e.cfg.Plans[tenantData.Plan]
	if !ok {
		return PolicyDecision{Allowed: false, Reason: ReasonInternalError} // Plan not found in config
	}

	groupCfg, ok := plan.Groups[matchedRoute.PlanGroup]
	if !ok || groupCfg.Limit == 0 {
		// No limit defined for this route group
		return PolicyDecision{
			Allowed:  true,
			Reason:   ReasonAllowed,
			RouteID:  matchedRoute.ID,
			Plan:     tenantData.Plan,
			TenantID: tenantData.TenantID,
		}
	}

	divisor := int64(60) // default minute
	if groupCfg.Unit == "second" {
		divisor = 1
	} else if groupCfg.Unit == "hour" {
		divisor = 3600
	}

	e.rateLimiter.Track(tenantData.TenantID, matchedRoute.ID, plan.ShardCount, divisor)
	allowed, remaining := e.rateLimiter.CheckAndRecord(ctx, tenantData.TenantID, matchedRoute.ID, plan.ShardCount, groupCfg.Limit, divisor)
	if !allowed {
		return PolicyDecision{
			Allowed:   false,
			Reason:    ReasonRateLimited,
			Remaining: 0,
			Reset:     time.Now().Add(time.Minute), // Dummy reset for now
			RouteID:   matchedRoute.ID,
			Plan:      tenantData.Plan,
			TenantID:  tenantData.TenantID,
		}
	}

	return PolicyDecision{
		Allowed:   true,
		Reason:    ReasonAllowed,
		Remaining: remaining,
		Reset:     time.Now().Add(time.Minute), // Dummy reset for now
		RouteID:   matchedRoute.ID,
		Plan:      tenantData.Plan,
		TenantID:  tenantData.TenantID,
	}
}

// matchPathPrefix is an inline helper to check path prefix without allocating
func matchPathPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

// ExtractEvaluationContext builds the evaluation context directly from the HTTP request 
// without escaping parameters to the heap.
func ExtractEvaluationContext(r *http.Request, apiKeyHeader string) EvaluationContext {
	return EvaluationContext{
		APIKey:   r.Header.Get(apiKeyHeader),
		Method:   r.Method,
		Path:     r.URL.Path,
		ClientIP: r.RemoteAddr,
	}
}
