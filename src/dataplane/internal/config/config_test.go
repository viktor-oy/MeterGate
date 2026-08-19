package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	yamlPath := filepath.Join(tempDir, "metergate.yml")

	yamlContent := `
server:
  port: 8080
  apiKeyHeader: x-api-key
proxy:
  enabled: true
  upstreamUrl: http://localhost:8000
plans:
  free:
    displayName: Free Tier
    shardCount: 1
    groups:
      api:
        limit: 10
        unit: minute
protectedRoutes:
  - id: api
    method: GET
    pathPrefix: /api
    upstreamPath: /api
    planGroup: api
`
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	cfg, err := config.LoadConfig(yamlPath)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "x-api-key", cfg.Server.ApiKeyHeader)
	assert.True(t, cfg.Proxy.Enabled)
	
	assert.Len(t, cfg.Plans, 1)
	assert.Equal(t, "Free Tier", cfg.Plans["free"].DisplayName)
	assert.Equal(t, 1, cfg.Plans["free"].ShardCount)

	assert.Len(t, cfg.ProtectedRoutes, 1)
	assert.Equal(t, "api", cfg.ProtectedRoutes[0].ID)
}

func TestInitRedisClient_FailFast(t *testing.T) {
	// Unset REDIS_HOST to trigger fail-fast behavior
	os.Unsetenv("REDIS_HOST")
	
	_, err := config.InitRedisClient()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS_HOST environment variable is required")
}

func TestLoadConfig_MissingUpstreamUrl(t *testing.T) {
	tempDir := t.TempDir()
	yamlPath := filepath.Join(tempDir, "metergate-missing.yml")

	yamlContent := `
server:
  port: 8080
proxy:
  enabled: true
  upstreamUrl: ""
`
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	_, err = config.LoadConfig(yamlPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upstreamUrl is required")
}

func TestLoadConfig_AllDisabled(t *testing.T) {
	tempDir := t.TempDir()
	yamlPath := filepath.Join(tempDir, "metergate-alldisabled.yml")

	yamlContent := `
server:
  port: 8080
proxy:
  enabled: false
provider:
  enabled: false
`
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	_, err = config.LoadConfig(yamlPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of proxy or provider mode must be enabled")
}
