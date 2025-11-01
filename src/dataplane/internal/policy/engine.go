package policy

import (
	"context"
	"net/http"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
)

// Reason string constants for policy decisions
const (
	ReasonAllowed          = "allowed"
	ReasonBlacklisted      = "blacklisted"
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
	cfg *config.MeterGateConfig
	// L1 Cache and Rate Limiter interfaces will be injected here in later stages
}

func NewCoreEngine(cfg *config.MeterGateConfig) *CoreEngine {
	return &CoreEngine{
		cfg: cfg,
	}
}

// Evaluate performs the zero-allocation chain of policy checks.
func (e *CoreEngine) Evaluate(ctx context.Context, evalCtx EvaluationContext) PolicyDecision {
	// 1. Missing API Key Check
	if evalCtx.APIKey == "" {
		return PolicyDecision{
			Allowed: false,
			Reason:  ReasonMissingHeader,
		}
	}

	// 2. Route Matching Check
	var matchedRoute *config.RouteConfig
	for i := range e.cfg.ProtectedRoutes {
		route := &e.cfg.ProtectedRoutes[i]
		if route.Method == evalCtx.Method && matchPathPrefix(evalCtx.Path, route.PathPrefix) {
			matchedRoute = route
			break
		}
	}

	if matchedRoute == nil {
		return PolicyDecision{
			Allowed: false,
			Reason:  ReasonRouteNotFound,
		}
	}

	// 3. Blacklist / API Key / Rate Limit checks will be implemented in later stages
	// For now, assume allowed if the route exists (Dummy implementation)
	return PolicyDecision{
		Allowed:   true,
		Reason:    ReasonAllowed,
		Remaining: 999, // Dummy value
		Reset:     time.Now().Add(time.Minute), // Dummy value
		RouteID:   matchedRoute.ID,
		Plan:      "dummy",
		TenantID:  "dummy",
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
