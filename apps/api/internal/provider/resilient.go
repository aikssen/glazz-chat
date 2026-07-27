package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/logging"
)

var ErrCircuitOpen = &Error{Code: CodeUnavailable, Retryable: true, Cause: errors.New("provider circuit is open")}

type ResilienceOptions struct {
	MaxConcurrent  int
	FailureLimit   int
	OpenDuration   time.Duration
	PreStreamRetry int
}

func DefaultResilienceOptions() ResilienceOptions {
	return ResilienceOptions{
		MaxConcurrent: 8, FailureLimit: 5, OpenDuration: 30 * time.Second, PreStreamRetry: 1,
	}
}

type Resilient struct {
	next      Gateway
	options   ResilienceOptions
	slots     chan struct{}
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	now       func() time.Time
	logger    *slog.Logger
}

func NewResilient(next Gateway, options ResilienceOptions) *Resilient {
	defaults := DefaultResilienceOptions()
	if options.MaxConcurrent <= 0 {
		options.MaxConcurrent = defaults.MaxConcurrent
	}
	if options.FailureLimit <= 0 {
		options.FailureLimit = defaults.FailureLimit
	}
	if options.OpenDuration <= 0 {
		options.OpenDuration = defaults.OpenDuration
	}
	if options.PreStreamRetry < 0 {
		options.PreStreamRetry = 0
	}
	return &Resilient{
		next: next, options: options, slots: make(chan struct{}, options.MaxConcurrent),
		now: time.Now, logger: slog.Default().With("component", "provider"),
	}
}

func (gateway *Resilient) WithLogger(logger *slog.Logger) *Resilient {
	if logger != nil {
		gateway.logger = logger.With("component", "provider")
	}
	return gateway
}

func (gateway *Resilient) Catalog(ctx context.Context) ([]Model, error) {
	logger := logging.Context(gateway.logger, ctx)
	logger.DebugContext(ctx, "provider catalog request started")
	if gateway.isOpen() {
		logger.WarnContext(ctx, "provider catalog blocked by open circuit")
		return nil, ErrCircuitOpen
	}
	models, err := gateway.next.Catalog(ctx)
	gateway.observe(err)
	if err != nil {
		logger.WarnContext(ctx, "provider catalog request failed",
			"error_type", fmt.Sprintf("%T", err),
		)
	} else {
		logger.InfoContext(ctx, "provider catalog request completed", "model_count", len(models))
	}
	return models, err
}

func (gateway *Resilient) Health(ctx context.Context) error {
	logger := logging.Context(gateway.logger, ctx)
	if gateway.isOpen() {
		logger.WarnContext(ctx, "provider health check blocked by open circuit")
		return ErrCircuitOpen
	}
	err := gateway.next.Health(ctx)
	gateway.observe(err)
	if err != nil {
		logger.WarnContext(ctx, "provider health check failed",
			"error_type", fmt.Sprintf("%T", err),
		)
	} else {
		logger.DebugContext(ctx, "provider health check completed")
	}
	return err
}

func (gateway *Resilient) Stream(ctx context.Context, request Request) (Stream, error) {
	logger := logging.Context(gateway.logger, ctx).With("provider_model", request.Model)
	logger.DebugContext(ctx, "provider stream request started")
	if gateway.isOpen() {
		logger.WarnContext(ctx, "provider stream blocked by open circuit")
		return nil, ErrCircuitOpen
	}
	select {
	case gateway.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var stream Stream
	var err error
	for attempt := 0; attempt <= gateway.options.PreStreamRetry; attempt++ {
		stream, err = gateway.next.Stream(ctx, request)
		if err == nil || !Normalize(err).Retryable {
			break
		}
		logger.WarnContext(ctx, "provider stream request retrying",
			"attempt", attempt+1,
			"error_code", Normalize(err).Code,
		)
	}
	if err != nil {
		<-gateway.slots
		gateway.observe(err)
		logger.WarnContext(ctx, "provider stream request failed",
			"error_code", Normalize(err).Code,
			"error_type", fmt.Sprintf("%T", err),
		)
		return nil, err
	}
	gateway.observe(nil)
	logger.InfoContext(ctx, "provider stream request connected")
	return &resilientStream{
		Stream:  stream,
		release: func() { <-gateway.slots },
		observe: gateway.observe,
	}, nil
}

func (gateway *Resilient) isOpen() bool {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.openUntil.IsZero() {
		return false
	}
	if gateway.now().Before(gateway.openUntil) {
		return true
	}
	gateway.openUntil = time.Time{}
	gateway.failures = 0
	return false
}

func (gateway *Resilient) observe(err error) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if err == nil {
		gateway.failures = 0
		gateway.openUntil = time.Time{}
		return
	}
	if !Normalize(err).Retryable {
		return
	}
	gateway.failures++
	if gateway.failures >= gateway.options.FailureLimit {
		gateway.openUntil = gateway.now().Add(gateway.options.OpenDuration)
	}
}

type resilientStream struct {
	Stream
	once    sync.Once
	release func()
	observe func(error)
}

func (stream *resilientStream) Next(ctx context.Context) (Chunk, error) {
	chunk, err := stream.Stream.Next(ctx)
	if err != nil {
		if errors.Is(err, io.EOF) {
			stream.observe(nil)
		} else {
			stream.observe(err)
		}
		stream.once.Do(stream.release)
	}
	return chunk, err
}

func (stream *resilientStream) Close() error {
	stream.once.Do(stream.release)
	return stream.Stream.Close()
}

var _ Gateway = (*Resilient)(nil)
