package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHTTPLogger(t *testing.T) {
	t.Run("logs 200 OK as Info", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, nil)
		log := slog.New(h)

		middleware := NewHTTPLogger(log)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("User-Agent", "Test-Agent")

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "INFO", logEntry["level"])
		assert.Equal(t, "HTTP Request", logEntry["msg"])
		assert.Equal(t, "GET", logEntry["method"])
		assert.Equal(t, "/test", logEntry["path"])
		assert.Equal(t, "HTTP/1.1", logEntry["protocol"])
		assert.Equal(t, "127.0.0.1:1234", logEntry["remote_ip"])
		assert.Equal(t, "Test-Agent", logEntry["user_agent"])
		assert.Equal(t, float64(http.StatusOK), logEntry["status"])
		assert.Equal(t, float64(2), logEntry["bytes"]) // "OK" is 2 bytes
		assert.NotNil(t, logEntry["duration"])
	})

	t.Run("logs 500 Internal Server Error as Error", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, nil)
		log := slog.New(h)

		middleware := NewHTTPLogger(log)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error occurred"))
		}))

		req := httptest.NewRequest(http.MethodPost, "/error-path", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "ERROR", logEntry["level"])
		assert.Equal(t, "HTTP Request", logEntry["msg"])
		assert.Equal(t, "POST", logEntry["method"])
		assert.Equal(t, "/error-path", logEntry["path"])
		assert.Equal(t, float64(http.StatusInternalServerError), logEntry["status"])
		assert.Equal(t, float64(14), logEntry["bytes"]) // "error occurred" is 14 bytes
	})

	t.Run("uses default logger when nil is provided", func(t *testing.T) {
		var buf bytes.Buffer

		// Capture standard output for the default logger
		h := slog.NewTextHandler(&buf, nil)
		slog.SetDefault(slog.New(h))

		middleware := NewHTTPLogger(nil)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPut, "/default", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Contains(t, buf.String(), "HTTP Request")
		assert.Contains(t, buf.String(), "method=PUT")
		assert.Contains(t, buf.String(), "path=/default")
		assert.Contains(t, buf.String(), "status=201")
	})

	t.Run("defaults to 200 OK if WriteHeader is not called", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, nil)
		log := slog.New(h)

		middleware := NewHTTPLogger(log)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("no status code set"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/no-status", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "INFO", logEntry["level"])
		assert.Equal(t, float64(http.StatusOK), logEntry["status"])
		assert.Equal(t, float64(18), logEntry["bytes"])
	})

	t.Run("uses X-Forwarded-For if available", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, nil)
		log := slog.New(h)

		middleware := NewHTTPLogger(log)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/proxy", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("X-Forwarded-For", "203.0.113.195, 198.51.100.1")

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "203.0.113.195", logEntry["remote_ip"])
	})

	t.Run("uses X-Real-IP if available", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, nil)
		log := slog.New(h)

		middleware := NewHTTPLogger(log)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/proxy", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("X-Real-IP", "203.0.113.195")

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "203.0.113.195", logEntry["remote_ip"])
	})

	t.Run("defaults to 200 OK if neither WriteHeader nor Write is called", func(t *testing.T) {
		var buf bytes.Buffer
		h := slog.NewJSONHandler(&buf, nil)
		log := slog.New(h)

		middleware := NewHTTPLogger(log)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Do nothing
		}))

		req := httptest.NewRequest(http.MethodGet, "/empty", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "INFO", logEntry["level"])
		assert.Equal(t, float64(http.StatusOK), logEntry["status"])
		assert.Equal(t, float64(0), logEntry["bytes"])
	})
}
