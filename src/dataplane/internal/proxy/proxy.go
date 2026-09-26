package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"unsafe"
)

// MeterGateProxy wraps httputil.ReverseProxy and implements buffer pooling.
type MeterGateProxy struct {
	ReverseProxy     *httputil.ReverseProxy
	DisableZeroAlloc bool
}

// pooledBuffer is a sync.Pool that provides reusable byte arrays.
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate a 32KB buffer, matching standard io.Copy buffers.
		return new([32 * 1024]byte)
	},
}

// bufferPoolAdapter adapts our sync.Pool to httputil.BufferPool interface.
type bufferPoolAdapter struct{}

func (b *bufferPoolAdapter) Get() []byte {
	arrPtr := bufferPool.Get().(*[32 * 1024]byte)
	return (*arrPtr)[:]
}

func (b *bufferPoolAdapter) Put(buf []byte) {
	// Reconstruct the array pointer from the slice's underlying data pointer.
	// This guarantees that the stack-allocated slice header `buf` does not escape to the heap.
	arrPtr := (*[32 * 1024]byte)(unsafe.Pointer(unsafe.SliceData(buf)))
	bufferPool.Put(arrPtr)
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
