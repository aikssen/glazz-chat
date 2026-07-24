package provider

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
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
		now: time.Now,
	}
}

func (gateway *Resilient) Catalog(ctx context.Context) ([]Model, error) {
	if gateway.isOpen() {
		return nil, ErrCircuitOpen
	}
	models, err := gateway.next.Catalog(ctx)
	gateway.observe(err)
	return models, err
}

func (gateway *Resilient) Stream(ctx context.Context, request Request) (Stream, error) {
	if gateway.isOpen() {
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
	}
	if err != nil {
		<-gateway.slots
		gateway.observe(err)
		return nil, err
	}
	gateway.observe(nil)
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
