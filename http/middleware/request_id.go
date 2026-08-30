package middleware

import (
	"net/http"
	"uuid"

	"github.com/pushkar-anand/build-with-go/ctxval"
)

// generateID generates a random UUID v7 string.
//
// V7 leads with a millisecond timestamp, so IDs sort chronologically. That is
// worth having once request IDs reach log aggregation, or are stored as keys.
func generateID() string {
	return uuid.NewV7().String()
}

// RequestID is a middleware that injects a request ID into the context of each request.
// If the incoming request has an "X-Request-Id" header, it is used; otherwise a new one is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateID()
		}

		ctx := ctxval.WithRequestID(r.Context(), reqID)
		w.Header().Set("X-Request-Id", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
