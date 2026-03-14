package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/pushkar-anand/build-with-go/logger"
)

// generateID generates a random 16-byte hex string.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// In the extremely rare case that rand.Read fails,
		// panic to ensure we don't proceed with an all-zero ID.
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// RequestID is a middleware that injects a request ID into the context of each request.
// If the incoming request has an "X-Request-Id" header, it is used; otherwise a new one is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateID()
		}

		ctx := logger.WithRequestID(r.Context(), reqID)
		w.Header().Set("X-Request-Id", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
