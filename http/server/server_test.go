package server

import (
	"context"
	"net/http"
	"testing"
)

func TestServer_Serve(t *testing.T) {
	t.Run("with default options", func(t *testing.T) {
		s := New(getTestHandler())

		ctx := context.Background()

		err := s.Serve(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func getTestHandler() http.Handler {
	m := http.NewServeMux()

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return m
}
