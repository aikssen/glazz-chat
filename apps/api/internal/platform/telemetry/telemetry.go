package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/logging"
)

type Runtime struct {
	tracerProvider *sdktrace.TracerProvider
	registry       *prometheus.Registry
	requests       *prometheus.CounterVec
	duration       *prometheus.HistogramVec
}

func New(ctx context.Context, cfg config.Telemetry) (*Runtime, error) {
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		)),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	if cfg.OTLPEndpoint != "" {
		exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint))
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	return NewWithProvider(provider), nil
}

func NewWithProvider(provider *sdktrace.TracerProvider) *Runtime {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "glazz",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "HTTP requests by bounded route, method, and status.",
	}, []string{"route", "method", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "glazz",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration by bounded route and method.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"route", "method"})
	registry.MustRegister(requests, duration)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Runtime{
		tracerProvider: provider,
		registry:       registry,
		requests:       requests,
		duration:       duration,
	}
}

func (runtime *Runtime) Shutdown(ctx context.Context) error {
	return runtime.tracerProvider.Shutdown(ctx)
}

func (runtime *Runtime) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(runtime.registry, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: 5,
	})
}

func (runtime *Runtime) Middleware(logger *slog.Logger) func(http.Handler) http.Handler {
	instrument := otelhttp.NewMiddleware(
		"glazz.http",
		otelhttp.WithTracerProvider(runtime.tracerProvider),
		otelhttp.WithPropagators(otel.GetTextMapPropagator()),
	)
	return func(next http.Handler) http.Handler {
		return instrument(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			started := time.Now()
			logger.DebugContext(
				request.Context(),
				"http request started",
				"request_id", httpx.RequestID(request.Context()),
				"correlation_id", logging.CorrelationID(request.Context()),
				"method", request.Method,
			)
			recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
			next.ServeHTTP(recorder, request)

			route := httpx.RoutePattern(request)
			status := strconv.Itoa(recorder.status)
			runtime.requests.WithLabelValues(route, request.Method, status).Inc()
			runtime.duration.WithLabelValues(route, request.Method).Observe(time.Since(started).Seconds())

			spanContext := trace.SpanContextFromContext(request.Context())
			logger.InfoContext(
				request.Context(),
				"http request",
				"request_id", httpx.RequestID(request.Context()),
				"correlation_id", logging.CorrelationID(request.Context()),
				"trace_id", spanContext.TraceID().String(),
				"method", request.Method,
				"route", route,
				"status", recorder.status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		}))
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}
