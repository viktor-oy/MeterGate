//go:build integration
// +build integration

package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestIntegration_DataPlaneBinary(t *testing.T) {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost:6379"
	}

	// 1. Build the binary
	cmdBuild := exec.Command("go", "build", "-o", "test-dataplane", "../../cmd/dataplane/main.go")
	cmdBuild.Stderr = os.Stderr
	cmdBuild.Stdout = os.Stdout
	if err := cmdBuild.Run(); err != nil {
		t.Fatalf("Failed to build dataplane binary: %v", err)
	}
	defer os.Remove("test-dataplane")

	// 0. Seed Redis L2
	opts, _ := redis.ParseURL("redis://" + redisHost)
	redisClient := redis.NewClient(opts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create seed hash
	apiKey := "dummy_key"
	secret := "local-demo-secret-change-me"
	h := sha256.New()
	h.Write([]byte(secret + ":" + apiKey))
	keyHash := hex.EncodeToString(h.Sum(nil))

	tenantData := map[string]interface{}{
		"tenantId": "test-tenant",
		"planCode": "free",
		"revoked":  false,
	}
	val, _ := json.Marshal(tenantData)
	redisClient.Set(ctx, "metergate:cache:apikey:"+keyHash, val, 0)

	// 1. Mock Upstream Server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock upstream response"))
	}))
	defer upstream.Close()

	// 2. Write a dummy config file
	configContent := `
server:
  port: 8089
  apiKeyHeader: "x-api-key"
proxy:
  enabled: true
  upstreamUrl: "` + upstream.URL + `"
  upstreamTimeoutMs: 5000
provider:
  enabled: true
  port: 8090
protectedRoutes:
  - id: "route_post"
    method: "POST"
    pathPrefix: "/post"
    planGroup: "standard"
plans:
  free:
    displayName: "Free Tier"
    shardCount: 1
    groups:
      standard:
        limit: 10
        unit: "minute"
`
	configPath := filepath.Join(t.TempDir(), "metergate.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// 3. Start the binary
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	cmd := exec.CommandContext(runCtx, "./test-dataplane")
	cmd.Env = append(os.Environ(), 
		"METERGATE_CONFIG_PATH="+configPath,
		"REDIS_HOST="+redisHost,
		"DISABLE_ZERO_ALLOC=false",
	)
	
	// We capture output to help debugging if it fails
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dataplane binary: %v", err)
	}

	// Wait for server to be ready
	time.Sleep(2 * time.Second)
	
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get("http://localhost:6060/health")
		if err != nil {
			t.Fatalf("Health check failed to connect: %v\nOutput: %s", err, out.String())
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health check returned status %v, expected 200", resp.StatusCode)
		}
	})

	executeCheck := func(t *testing.T, mode, apiKey string, expectedStatus int, ignoreStatus bool) {
		var resp *http.Response
		var err error

		if mode == "Provider" {
			reqBody := `{"apiKey": "` + apiKey + `", "method": "POST", "path": "/post"}`
			resp, err = http.Post("http://localhost:8090/v1/check", "application/json", bytes.NewBufferString(reqBody))
		} else { // Proxy
			req, _ := http.NewRequest("POST", "http://localhost:8089/post", nil)
			if apiKey != "" {
				req.Header.Set("x-api-key", apiKey)
			}
			resp, err = http.DefaultClient.Do(req)
		}
		
		if err != nil {
			t.Fatalf("[%s] request failed: %v", mode, err)
		}
		defer resp.Body.Close()

		if ignoreStatus {
			return
		}

		if resp.StatusCode != expectedStatus {
			t.Errorf("[%s] Expected status %v, got %v\nBinary Output: %s", mode, expectedStatus, resp.StatusCode, out.String())
		}

		// Additional check for Provider success
		if mode == "Provider" && expectedStatus == http.StatusOK {
			var decision map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
				t.Fatalf("Failed to decode provider response: %v", err)
			}
			if allowed, ok := decision["Allowed"].(bool); !ok || !allowed {
				t.Errorf("Expected Allowed to be true, got %v", decision["Allowed"])
			}
		}
	}

	t.Run("Success", func(t *testing.T) {
		executeCheck(t, "Provider", "dummy_key", http.StatusOK, false)
		executeCheck(t, "Proxy", "dummy_key", http.StatusOK, false)
	})

	t.Run("MissingKey", func(t *testing.T) {
		executeCheck(t, "Provider", "", http.StatusUnauthorized, false)
		executeCheck(t, "Proxy", "", http.StatusUnauthorized, false)
	})

	t.Run("InvalidKey", func(t *testing.T) {
		executeCheck(t, "Provider", "wrong_key", http.StatusUnauthorized, false)
		executeCheck(t, "Proxy", "wrong_key", http.StatusUnauthorized, false)
	})

	t.Run("RateLimitExhaustion", func(t *testing.T) {
		// Fire 15 requests via Provider to exhaust the limit of 10
		for i := 0; i < 15; i++ {
			executeCheck(t, "Provider", "dummy_key", 0, true)
		}

		// Wait for the background aggregator to sweep and sync state to Redis/L1
		time.Sleep(300 * time.Millisecond)

		// Both modes should now be rate limited
		executeCheck(t, "Provider", "dummy_key", http.StatusTooManyRequests, false)
		executeCheck(t, "Proxy", "dummy_key", http.StatusTooManyRequests, false)
	})

	// Clean shutdown
	runCancel()
	cmd.Wait()
}
