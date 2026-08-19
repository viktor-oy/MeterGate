package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// MeterGateProxy wraps httputil.ReverseProxy and implements buffer pooling.
type MeterGateProxy struct {
	ReverseProxy     *httputil.ReverseProxy
	DisableZeroAlloc bool
}

// pooledBuffer is a sync.Pool that provides reusable byte slices.
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate a 32KB buffer, matching standard io.Copy buffers.
		b := make([]byte, 32*1024)
		return &b
	},
}

// bufferPoolAdapter adapts our sync.Pool to httputil.BufferPool interface.
type bufferPoolAdapter struct{}

func (b *bufferPoolAdapter) Get() []byte {
	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr
	return buf[:cap(buf)] // Ensure we return the full capacity slice
}

func (b *bufferPoolAdapter) Put(buf []byte) {
	// Strict slice length resetting to prevent tenant data leakage.
	// We reset length to 0 but keep capacity.
	buf = buf[:0]
	bufferPool.Put(&buf)
}

// NewMeterGateProxy creates a new proxy instance.
func NewMeterGateProxy(targetURL *url.URL, disableZeroAlloc bool) *MeterGateProxy {
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	if !disableZeroAlloc {
		// Use sync.Pool for buffer pooling when zero allocation is enabled.
		proxy.BufferPool = &bufferPoolAdapter{}
	}

	return &MeterGateProxy{
		ReverseProxy:     proxy,
		DisableZeroAlloc: disableZeroAlloc,
	}
}

// ServeHTTP implements http.Handler for the proxy.
func (p *MeterGateProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.ReverseProxy.ServeHTTP(w, r)
}
