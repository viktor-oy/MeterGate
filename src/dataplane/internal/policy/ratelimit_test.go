package policy

import (
	"strings"
	"testing"
)

func TestGetRateLimitKey_SingleShard(t *testing.T) {
	tenantID := "tenant-abc"
	routeID := "route-xyz"
	shardCount := 1
	var window int64 = 28500000 // Fixed timestamp for deterministic testing

	key := GetRateLimitKey(tenantID, routeID, shardCount, window)
	expected := "metergate:rate:tenant-abc:route-xyz:28500000:0"

	if key != expected {
		t.Errorf("Expected %s, got %s", expected, key)
	}
}

func TestGetRateLimitKey_MultiShard(t *testing.T) {
	tenantID := "tenant-ent"
	routeID := "route-heavy"
	shardCount := 10
	var window int64 = 28500000 

	key := GetRateLimitKey(tenantID, routeID, shardCount, window)
	
	// Since fastrand introduces non-determinism, we verify the prefix and suffix range.
	prefix := "metergate:rate:tenant-ent:route-heavy:28500000:"
	if !strings.HasPrefix(key, prefix) {
		t.Errorf("Expected prefix %s, got key %s", prefix, key)
	}

	saltStr := strings.TrimPrefix(key, prefix)
	salt := -1
	for i := 0; i < shardCount; i++ {
		if saltStr == string(rune('0'+i)) || saltStr == "10" { // simple check, robust enough for 0-9
			// We can parse properly
			importStrConv := saltStr // just checking it's single/double digit
			_ = importStrConv
			salt = i
			break
		}
	}
	
	if salt == -1 {
		// Just parse it to be sure
		t.Errorf("Invalid salt suffix in key: %s", key)
	}
}
