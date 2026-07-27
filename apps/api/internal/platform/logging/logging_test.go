package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLoggerFiltersLevelsAndAddsCorrelation(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, "warn", "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithCorrelationID(context.Background(), "corr-test-1")
	Context(logger, ctx).InfoContext(ctx, "hidden")
	Context(logger, ctx).WarnContext(ctx, "visible")

	logs := output.String()
	if strings.Contains(logs, "hidden") || !strings.Contains(logs, "visible") ||
		!strings.Contains(logs, `"correlation_id":"corr-test-1"`) {
		t.Fatalf("unexpected logs: %s", logs)
	}
}

func TestLoggerRejectsInvalidLevel(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "verbose", "test"); err == nil {
		t.Fatal("New() error = nil")
	}
}
