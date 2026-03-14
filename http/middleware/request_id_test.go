package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkar-anand/build-with-go/logger"
)

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, ok := logger.RequestIDFromContext(r.Context())
		if !ok {
			t.Error("expected request ID in context")
		}
		if reqID == "" {
			t.Error("expected non-empty request ID")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rr.Code)
	}

	headerID := rr.Header().Get("X-Request-Id")
	if headerID == "" {
		t.Error("expected X-Request-Id header")
	}
}

func TestRequestID_ExistingHeader(t *testing.T) {
	existingID := "existing-req-id"
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, ok := logger.RequestIDFromContext(r.Context())
		if !ok {
			t.Error("expected request ID in context")
		}
		if reqID != existingID {
			t.Errorf("expected request ID %q in context, got %q", existingID, reqID)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", existingID)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rr.Code)
	}

	headerID := rr.Header().Get("X-Request-Id")
	if headerID != existingID {
		t.Errorf("expected X-Request-Id header %q, got %q", existingID, headerID)
	}
}
