package logger

import (
	"context"
	"testing"
)

func TestContextRequestID(t *testing.T) {
	ctx := context.Background()

	// Should not have request ID initially
	_, ok := RequestIDFromContext(ctx)
	if ok {
		t.Error("expected no request ID in empty context")
	}

	reqID := "test-request-id-123"
	ctx = WithRequestID(ctx, reqID)

	val, ok := RequestIDFromContext(ctx)
	if !ok {
		t.Error("expected to find request ID in context")
	}
	if val != reqID {
		t.Errorf("expected %q, got %q", reqID, val)
	}
}
