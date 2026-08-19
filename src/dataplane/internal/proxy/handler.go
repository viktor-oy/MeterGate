package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/policy"
)

// ProxyHandler handles all incoming HTTP traffic for the Reverse Proxy.
type ProxyHandler struct {
	engine policy.Engine
	proxy  *MeterGateProxy
}

func NewProxyHandler(engine policy.Engine, proxy *MeterGateProxy) *ProxyHandler {
	return &ProxyHandler{
		engine: engine,
		proxy:  proxy,
	}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	apiKey := extractAPIKey(r)
	evalCtx := policy.EvaluationContext{
		APIKey:   apiKey,
		Method:   r.Method,
		Path:     r.URL.Path,
		ClientIP: r.RemoteAddr,
	}

	decision := h.engine.Evaluate(r.Context(), evalCtx)
	writeRateLimitHeaders(w, decision)

	if !decision.Allowed {
		status := reasonToStatus(decision.Reason)
		http.Error(w, decision.Reason, status)
		return
	}

	// Forward the request to the upstream via our proxy
	h.proxy.ServeHTTP(w, r)
}

// ProviderHandler handles explicitly the /v1/check endpoint.
type ProviderHandler struct {
	engine policy.Engine
}

func NewProviderHandler(engine policy.Engine) *ProviderHandler {
	return &ProviderHandler{
		engine: engine,
	}
}

type checkRequest struct {
	APIKey string `json:"apiKey"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

func (h *ProviderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/check" {
		http.NotFound(w, r)
		return
	}

	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	evalCtx := policy.EvaluationContext{
		APIKey:   req.APIKey,
		Method:   req.Method,
		Path:     req.Path,
		ClientIP: r.RemoteAddr,
	}

	decision := h.engine.Evaluate(r.Context(), evalCtx)

	w.Header().Set("Content-Type", "application/json")
	writeRateLimitHeaders(w, decision)

	if !decision.Allowed {
		w.WriteHeader(reasonToStatus(decision.Reason))
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(decision)
}

// Shared helpers
func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return r.Header.Get("x-api-key")
}

func writeRateLimitHeaders(w http.ResponseWriter, d policy.PolicyDecision) {
	if d.Plan != "" {
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(d.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(d.Reset.Unix(), 10))
	}
}

func reasonToStatus(reason string) int {
	switch reason {
	case policy.ReasonMissingHeader, policy.ReasonInvalidKey:
		return http.StatusUnauthorized
	case policy.ReasonBlacklisted:
		return http.StatusForbidden
	case policy.ReasonRateLimited:
		return http.StatusTooManyRequests
	case policy.ReasonRouteNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
