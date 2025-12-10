package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/policy"
)

// Handler handles all incoming HTTP traffic to the Data Plane.
type Handler struct {
	engine     policy.Engine
	proxy      *MeterGateProxy
}

func NewHandler(engine policy.Engine, proxy *MeterGateProxy) *Handler {
	return &Handler{
		engine: engine,
		proxy:  proxy,
	}
}

// checkRequest payload for POST /v1/check
type checkRequest struct {
	APIKey string `json:"apiKey"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

// ServeHTTP acts as the router.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/v1/check" {
		h.handleProviderMode(w, r)
		return
	}

	h.handleProxyMode(w, r)
}

func (h *Handler) handleProviderMode(w http.ResponseWriter, r *http.Request) {
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

	// For Provider Mode, use the request context directly.
	decision := h.engine.Evaluate(r.Context(), evalCtx)

	w.Header().Set("Content-Type", "application/json")
	h.writeRateLimitHeaders(w, decision)

	if !decision.Allowed {
		w.WriteHeader(h.reasonToStatus(decision.Reason))
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(decision)
}

func (h *Handler) handleProxyMode(w http.ResponseWriter, r *http.Request) {
	// Extract evaluation context without allocating strings when possible
	apiKey := h.extractAPIKey(r)
	evalCtx := policy.EvaluationContext{
		APIKey:   apiKey,
		Method:   r.Method,
		Path:     r.URL.Path,
		ClientIP: r.RemoteAddr,
	}

	// We pass a background context to avoid cancelling background rate-limiting writes
	// if the client disconnects, though in this design Evaluate is zero-alloc and synchronous
	// except for Redis write which happens inside the Engine.
	// We'll use the request context.
	decision := h.engine.Evaluate(r.Context(), evalCtx)

	h.writeRateLimitHeaders(w, decision)

	if !decision.Allowed {
		status := h.reasonToStatus(decision.Reason)
		http.Error(w, decision.Reason, status)
		return
	}

	// Forward the request to the upstream via our proxy
	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return r.Header.Get("x-api-key")
}

func (h *Handler) writeRateLimitHeaders(w http.ResponseWriter, d policy.PolicyDecision) {
	if d.Plan != "" {
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(d.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(d.Reset.Unix(), 10))
	}
}

func (h *Handler) reasonToStatus(reason string) int {
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
