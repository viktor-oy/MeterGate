package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anahvictoronyedikachi/metergate/src/dataplane/internal/config"
)

func TestL1Cache_TenantData(t *testing.T) {
	c := NewL1Cache()

	key := "test-api-key"
	expectedData := TenantData{TenantID: "tenant-123", Plan: "pro"}

	// Test Get on empty
	_, found := c.GetTenantData(key)
	if found {
		t.Errorf("Expected not found on empty cache")
	}

	// Test Set and Get
	c.SetTenantData(key, expectedData)
	data, found := c.GetTenantData(key)
	if !found {
		t.Fatalf("Expected to find key after Set")
	}
	if data != expectedData {
		t.Errorf("Expected %v, got %v", expectedData, data)
	}
}

func TestL1Cache_RouteRCU(t *testing.T) {
	c := NewL1Cache()

	// Initial should be empty but non-nil
	routes := c.LoadRoutes()
	if routes == nil {
		t.Fatalf("Expected non-nil initial routes")
	}
	if len(*routes) != 0 {
		t.Fatalf("Expected 0 initial routes, got %d", len(*routes))
	}

	// Update Routes
	newRoutes := []config.RouteConfig{
		{ID: "route-1", PathPrefix: "/v1/test"},
	}
	c.UpdateRoutes(newRoutes)

	// Verify Update
	updated := c.LoadRoutes()
	if len(*updated) != 1 {
		t.Fatalf("Expected 1 route after update, got %d", len(*updated))
	}
	if (*updated)[0].ID != "route-1" {
		t.Errorf("Expected route-1, got %s", (*updated)[0].ID)
	}
}

func TestL1Cache_FetchTenantWithSingleflight(t *testing.T) {
	c := NewL1Cache()
	apiKey := "singleflight-key"

	var fetchCount atomic.Int32
	fetchFn := func() (TenantData, error) {
		fetchCount.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow DB fetch
		return TenantData{TenantID: "tenant-slow", Plan: "enterprise"}, nil
	}

	// Launch multiple concurrent requests
	var wg sync.WaitGroup
	numRequests := 10
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			data, err := c.FetchTenantWithSingleflight(apiKey, fetchFn)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if data.TenantID != "tenant-slow" {
				t.Errorf("Expected tenant-slow, got %v", data.TenantID)
			}
		}()
	}

	wg.Wait()

	// Verify fetchFn was only called EXACTLY once thanks to singleflight
	if fetchCount.Load() != 1 {
		t.Errorf("Expected fetch to be called exactly 1 time, but was called %d times", fetchCount.Load())
	}
}

func TestL1Cache_FetchTenantWithSingleflight_ErrorPropagation(t *testing.T) {
	c := NewL1Cache()
	apiKey := "error-key"

	expectedErr := errors.New("database connection failed")
	fetchFn := func() (TenantData, error) {
		return TenantData{}, expectedErr
	}

	_, err := c.FetchTenantWithSingleflight(apiKey, fetchFn)
	if err != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}

	// Verify cache is still empty
	_, found := c.GetTenantData(apiKey)
	if found {
		t.Errorf("Expected cache to be empty after fetch failure")
	}
}
