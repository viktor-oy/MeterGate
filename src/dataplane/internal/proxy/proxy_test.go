package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMeterGateProxy_BufferPool(t *testing.T) {
	// Dummy upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	u, _ := url.Parse(upstream.URL)
	proxy := NewMeterGateProxy(u, false)

	if proxy.ReverseProxy.BufferPool == nil {
		t.Fatal("Expected BufferPool to be initialized when disableZeroAlloc is false")
	}

	// Test the buffer pool adapter
	adapter := proxy.ReverseProxy.BufferPool.(*bufferPoolAdapter)

	// Get a buffer, write to it, and put it back
	buf := adapter.Get()
	if len(buf) != 32*1024 {
		t.Fatalf("Expected buffer length to be 32KB, got %d", len(buf))
	}

	copy(buf, []byte("tenant A sensitive data"))
	
	// Put it back
	adapter.Put(buf)

	// Get a buffer again and ensure it's empty (len 0), though capacity remains
	buf2 := adapter.Get()
	
	// Our adapter returns buf[:cap(buf)] so len should be back to 32KB
	if len(buf2) != 32*1024 {
		t.Fatalf("Expected buffer length to be restored to 32KB, got %d", len(buf2))
	}

	// The important part is that we didn't just slice it to 0 and lose capacity
	if cap(buf2) != 32*1024 {
		t.Fatalf("Expected buffer capacity to be 32KB, got %d", cap(buf2))
	}
}

func TestMeterGateProxy_DisableZeroAlloc(t *testing.T) {
	u, _ := url.Parse("http://localhost:8080")
	proxy := NewMeterGateProxy(u, true)

	if proxy.ReverseProxy.BufferPool != nil {
		t.Fatal("Expected BufferPool to be nil when disableZeroAlloc is true")
	}
}
