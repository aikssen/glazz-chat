package telemetry

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

func TestRequestCorrelatesTraceMetricAndSafeLog(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	runtime := NewWithProvider(provider)
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := chi.NewRouter()
	router.Use(httpx.RequestIDs(ids.NewFake(uuid.MustParse("018f0000-0000-7000-8000-000000000001"))))
	router.Use(runtime.Middleware(logger))
	router.Get("/synthetic/{id}", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/synthetic/private-content", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if len(recorder.Ended()) != 1 {
		t.Fatalf("ended spans = %d", len(recorder.Ended()))
	}
	if strings.Contains(logs.String(), "private-content") {
		t.Fatalf("log contains raw path: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "/synthetic/{id}") ||
		!strings.Contains(logs.String(), "request_id") ||
		!strings.Contains(logs.String(), "trace_id") {
		t.Fatalf("log lacks correlation fields: %s", logs.String())
	}

	metrics := httptest.NewRecorder()
	runtime.MetricsHandler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metrics.Body.String(), `route="/synthetic/{id}"`) {
		t.Fatalf("metrics lack bounded route: %s", metrics.Body.String())
	}
}

func TestUnavailableCollectorDoesNotFailRequests(t *testing.T) {
	runtime, err := New(context.Background(), config.Telemetry{
		ServiceName:  "glazz-test",
		OTLPEndpoint: "http://127.0.0.1:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = runtime.Shutdown(ctx)
	})

	router := chi.NewRouter()
	router.Use(runtime.Middleware(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.Get("/health", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
