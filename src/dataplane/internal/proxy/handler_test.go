package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderHandler_InvalidPath(t *testing.T) {
	h := NewProviderHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/random-path", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %v", rr.Code)
	}
}
