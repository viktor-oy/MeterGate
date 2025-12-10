package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/cache"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/policy"
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/proxy"
)

func main() {
	configPath := os.Getenv("METERGATE_CONFIG_PATH")
	if configPath == "" {
		configPath = "/app/config/metergate.yml" // Default inside container
	}

	log.Printf("Loading configuration from %s", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	redisClient, err := config.InitRedisClient()
	if err != nil {
		log.Fatalf("Fail-Fast Redis Initialization Error: %v", err)
	}
	defer redisClient.Close()
	log.Println("Redis Client initialized successfully.")

	// Check for chaos feature flag
	disableZeroAllocStr := os.Getenv("DISABLE_ZERO_ALLOC")
	disableZeroAlloc, _ := strconv.ParseBool(disableZeroAllocStr)
	if disableZeroAlloc {
		log.Println("WARNING: DISABLE_ZERO_ALLOC chaos flag is ENABLED. sync.Pool is bypassed.")
	}

	// Initialize components
	l1Cache := cache.NewL1Cache()
	rateLimiter := policy.NewRateLimiter(redisClient)
	engine := policy.NewCoreEngine(cfg, l1Cache, rateLimiter, redisClient)

	// Parse upstream URL
	upstreamURL, err := url.Parse(cfg.Proxy.UpstreamUrl)
	if err != nil {
		log.Fatalf("Failed to parse upstream URL: %v", err)
	}

	// Initialize Proxy and Handler
	reverseProxy := proxy.NewMeterGateProxy(upstreamURL, disableZeroAlloc)
	handler := proxy.NewHandler(engine, reverseProxy)

	// Create main server
	serverPort := cfg.Server.Port
	if serverPort == 0 {
		serverPort = 8080
	}
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(serverPort),
		Handler: handler,
	}

	// Setup graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start background aggregator
	rateLimiter.StartBackgroundAggregator(ctx, 1)

	// Start internal diagnostics server on :6060 (Liveness probe logic here)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := redisClient.Ping(r.Context()).Err(); err != nil {
				http.Error(w, "Redis Disconnected", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		log.Println("Starting diagnostics server on :6060 (Liveness Probes & Metrics)")
		if err := http.ListenAndServe(":6060", mux); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Diagnostics server failed: %v", err)
		}
	}()

	// Start primary server
	go func() {
		log.Printf("Starting MeterGate Data Plane proxy on :%d", serverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	log.Println("Shutdown signal received, draining traffic...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server gracefully stopped.")
}
