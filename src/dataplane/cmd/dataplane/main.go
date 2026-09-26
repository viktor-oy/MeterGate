package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/pprof"
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
	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	telemetry.InitLogger(slog.LevelInfo)

	configPath := os.Getenv("METERGATE_CONFIG_PATH")
	if configPath == "" {
		configPath = "/app/config/metergate.yml" // Default inside container
	}

	slog.Info("Loading configuration", "path", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	redisClient, err := config.InitRedisClient()
	if err != nil {
		slog.Error("Fail-Fast Redis Initialization Error", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	slog.Info("Redis Client initialized successfully.")

	// Check for chaos feature flag
	disableZeroAllocStr := os.Getenv("DISABLE_ZERO_ALLOC")
	disableZeroAlloc, _ := strconv.ParseBool(disableZeroAllocStr)
	if disableZeroAlloc {
		slog.Warn("DISABLE_ZERO_ALLOC chaos flag is ENABLED. sync.Pool is bypassed.")
	}

	// Initialize components
	l1Cache := cache.NewL1Cache(cfg.Cache.L1.ApiKeyTtlSeconds)
	rateLimiter := policy.NewRateLimiter(redisClient)
	engine, err := policy.NewCoreEngine(cfg, l1Cache, rateLimiter, redisClient)
	if err != nil {
		slog.Error("Failed to initialize CoreEngine", "error", err)
		os.Exit(1)
	}

	// Track servers for graceful shutdown
	var servers []*http.Server

	upstreamURL, err := url.Parse(cfg.Proxy.UpstreamUrl)
	if err != nil {
		slog.Error("Failed to parse upstream URL", "error", err)
		os.Exit(1)
	}
	reverseProxy := proxy.NewMeterGateProxy(upstreamURL, disableZeroAlloc)
	proxyHandler := proxy.NewProxyHandler(engine, reverseProxy)

	serverPort := cfg.Server.Port
	if envPort := os.Getenv("METERGATE_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			serverPort = p
		}
	}
	if serverPort == 0 {
		serverPort = 8080 // default proxy port
	}
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(serverPort),
		Handler: proxyHandler,
	}
	servers = append(servers, srv)
	go func() {
		slog.Info("Starting MeterGate Data Plane", "port", serverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Proxy Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Setup graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start background aggregator and sweepers
	rateLimiter.StartBackgroundAggregator(ctx)
	l1Cache.StartSweeper(ctx)

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
		// Mount pprof
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		// Mount Prometheus metrics
		mux.Handle("/metrics", promhttp.Handler())

		metricsPort := os.Getenv("METERGATE_METRICS_PORT")
		if metricsPort == "" {
			metricsPort = "6060"
		}
		slog.Info("Starting diagnostics server", "port", metricsPort)
		if err := http.ListenAndServe(":"+metricsPort, mux); err != nil && err != http.ErrServerClosed {
			slog.Error("Diagnostics server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	slog.Info("Shutdown signal received, draining traffic...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server forced to shutdown", "error", err)
		}
	}
	slog.Info("All servers gracefully stopped.")
}
