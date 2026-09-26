package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type MeterGateConfig struct {
	Server          ServerConfig    `yaml:"server"`
	Proxy           ProxyConfig     `yaml:"proxy"`

	Cache           CacheConfig     `yaml:"cache"`
	ProtectedRoutes []RouteConfig   `yaml:"protectedRoutes"`
	Plans           map[string]Plan `yaml:"plans"`
}

type ServerConfig struct {
	Port         int    `yaml:"port"`
	ApiKeyHeader string `yaml:"apiKeyHeader"`
}

type ProxyConfig struct {
	UpstreamUrl       string `yaml:"upstreamUrl"`
	UpstreamTimeoutMs int    `yaml:"upstreamTimeoutMs"`
}


type CacheConfig struct {
	L1 L1CacheConfig `yaml:"l1"`
	L2 L2CacheConfig `yaml:"l2"`
}

type L1CacheConfig struct {
	MaxItems             int `yaml:"maxItems"`
	RouteTtlSeconds      int `yaml:"routeTtlSeconds"`
	ApiKeyTtlSeconds     int `yaml:"apiKeyTtlSeconds"`
	TenantPlanTtlSeconds int `yaml:"tenantPlanTtlSeconds"`
	ConfigTtlSeconds     int `yaml:"configTtlSeconds"`
}

type L2CacheConfig struct {
	ApiKeyTtlSeconds     int `yaml:"apiKeyTtlSeconds"`
	TenantPlanTtlSeconds int `yaml:"tenantPlanTtlSeconds"`
}

type RouteConfig struct {
	ID           string `yaml:"id"`
	Method       string `yaml:"method"`
	PathPrefix   string `yaml:"pathPrefix"`
	UpstreamPath string `yaml:"upstreamPath"`
	PlanGroup    string `yaml:"planGroup"`
}

type Plan struct {
	DisplayName string                       `yaml:"displayName"`
	ShardCount  int                          `yaml:"shardCount"`
	Groups      map[string]RateLimitGroupConfig `yaml:"groups"`
}

type RateLimitGroupConfig struct {
	Limit int    `yaml:"limit"`
	Unit  string `yaml:"unit"`
}



// LoadConfig parses the metergate.yml file.
func LoadConfig(path string) (*MeterGateConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg MeterGateConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	for planName, plan := range cfg.Plans {
		for groupName, group := range plan.Groups {
			if group.Unit != "second" && group.Unit != "minute" && group.Unit != "hour" {
				return nil, fmt.Errorf("invalid unit '%s' in plan '%s' group '%s': must be second, minute, or hour", group.Unit, planName, groupName)
			}
		}
	}



	if cfg.Proxy.UpstreamUrl == "" {
		return nil, fmt.Errorf("upstreamUrl is required in proxy configuration for fail-fast boot")
	}

	return &cfg, nil
}

// InitRedisClient initializes a fail-fast UniversalClient using REDIS_HOST.
func InitRedisClient() (redis.UniversalClient, error) {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		return nil, fmt.Errorf("REDIS_HOST environment variable is required for crash-only fail-fast boot")
	}

	addrs := strings.Split(redisHost, ",")

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: addrs,
	})

	// Verify connection immediately
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis during initialization: %w", err)
	}

	return client, nil
}
