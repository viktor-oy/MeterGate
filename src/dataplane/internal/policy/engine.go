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

func NewCoreEngine(cfg *config.MeterGateConfig, l1 *cache.L1Cache, rl *RateLimiter, rc redis.UniversalClient) *CoreEngine {
	secret := os.Getenv("METERGATE_KEY_HASH_SECRET")
	if secret == "" {
		secret = "local-demo-secret-change-me"
	}
	return &CoreEngine{
		cfg:         cfg,
		l1Cache:     l1,
		rateLimiter: rl,
		redisClient: rc,
		hashSecret:  secret,
	}
}

// hashAPIKey safely generates the sha256 hex string used as the key in Redis L2.
func (e *CoreEngine) hashAPIKey(apiKey string) string {
	h := sha256.New()
	h.Write([]byte(e.hashSecret + ":" + apiKey))
	return hex.EncodeToString(h.Sum(nil))
}

// Evaluate performs the zero-allocation chain of policy checks.
func (e *CoreEngine) Evaluate(ctx context.Context, evalCtx EvaluationContext) PolicyDecision {
	// 1. Missing API Key Check
	if evalCtx.APIKey == "" {
		return PolicyDecision{Allowed: false, Reason: ReasonMissingHeader}
	}

	keyHash := e.hashAPIKey(evalCtx.APIKey)

	// 2. Resolve Tenant from L1/L2
	tenantData, err := e.l1Cache.FetchTenantWithSingleflight(keyHash, func() (cache.TenantData, error) {
		// Fallback to Redis L2
		val, err := e.redisClient.Get(ctx, "metergate:cache:apikey:"+keyHash).Result()
		if err != nil {
			return cache.TenantData{}, err
		}
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

	e.rateLimiter.Track(tenantData.TenantID, matchedRoute.ID)
	allowed, remaining := e.rateLimiter.CheckAndRecord(ctx, tenantData.TenantID, matchedRoute.ID, plan.ShardCount, groupCfg.Limit)
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
