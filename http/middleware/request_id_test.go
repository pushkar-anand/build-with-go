package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/pushkar-anand/build-with-go/ctxval"
)

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, ok := ctxval.RequestIDFromContext(r.Context())
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
		reqID, ok := ctxval.RequestIDFromContext(r.Context())
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

func TestRequestID_GeneratesUUIDv7(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-Id")

	u, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("generated request ID %q is not a valid UUID: %v", id, err)
	}

	// The version lives in the high nibble of byte 6.
	if v := u[6] >> 4; v != 7 {
		t.Errorf("expected a v7 UUID, got v%d", v)
	}
}

func TestGenerateID_SortsChronologically(t *testing.T) {
	first := generateID()

	// A v7 timestamp has millisecond resolution, so step past one.
	time.Sleep(5 * time.Millisecond)

	second := generateID()

	if first >= second {
		t.Errorf("expected %q to sort before %q", first, second)
	}
}
