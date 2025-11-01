package policy_test

import (
	"context"
	"testing"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/policy"
	"github.com/stretchr/testify/assert"
)

func TestCoreEngine_Evaluate(t *testing.T) {
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
	}

	engine := policy.NewCoreEngine(cfg)
	ctx := context.Background()

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
	})
}
