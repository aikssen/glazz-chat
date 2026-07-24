package provider

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

type FakeOptions struct {
	Models    []Model
	Chunks    []string
	Latency   time.Duration
	FailAfter int
	Usage     Usage
}

type Fake struct {
	options FakeOptions
	mu      sync.Mutex
	calls   int
}

func NewFake(options FakeOptions) *Fake {
	if len(options.Models) == 0 {
		options.Models = []Model{{
			ID: "deepseek-v4-flash", ContextWindow: 131072,
			MaxOutputTokens: 8192, ChatCompletions: true,
		}}
	}
	if len(options.Chunks) == 0 {
		options.Chunks = []string{"Deterministic ", "development ", "response."}
	}
	if options.Usage == (Usage{}) {
		options.Usage = Usage{InputTokens: 8, OutputTokens: 6}
	}
	return &Fake{options: options}
}

func (fake *Fake) Catalog(context.Context) ([]Model, error) {
	return append([]Model(nil), fake.options.Models...), nil
}

func (fake *Fake) Health(ctx context.Context) error {
	return ctx.Err()
}

func (fake *Fake) Stream(ctx context.Context, request Request) (Stream, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	fake.mu.Lock()
	fake.calls++
	fake.mu.Unlock()
	return &fakeStream{
		ctx: ctx, options: fake.options, model: request.Model,
		maxOutputTokens: request.MaxOutputTokens,
	}, nil
}

func (fake *Fake) Calls() int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.calls
}

type fakeStream struct {
	ctx             context.Context
	options         FakeOptions
	model           string
	maxOutputTokens int
	index           int
	closed          bool
}

func (stream *fakeStream) Next(ctx context.Context) (Chunk, error) {
	if stream.closed {
		return Chunk{}, io.EOF
	}
	if stream.options.FailAfter > 0 && stream.index >= stream.options.FailAfter {
		stream.closed = true
		return Chunk{}, &Error{Code: CodeUnavailable, Retryable: false, Cause: io.ErrUnexpectedEOF}
	}
	if stream.index < len(stream.options.Chunks) {
		if err := wait(ctx, stream.ctx, stream.options.Latency); err != nil {
			return Chunk{}, err
		}
		text := stream.options.Chunks[stream.index]
		stream.index++
		return Chunk{Text: text, ProviderModel: stream.model}, nil
	}
	stream.closed = true
	usage := stream.options.Usage
	if usage.OutputTokens > stream.maxOutputTokens {
		usage.OutputTokens = stream.maxOutputTokens
	}
	return Chunk{
		Usage: &usage, FinishReason: FinishStop, ProviderModel: stream.model,
		RequestID: "fake-request",
	}, nil
}

func (stream *fakeStream) Close() error {
	stream.closed = true
	return nil
}

func validateRequest(request Request) error {
	if request.Model == "" || len(request.Messages) == 0 || request.MaxOutputTokens <= 0 {
		return &Error{Code: CodeInvalidRequest, Cause: errors.New("model, messages, and max output tokens are required")}
	}
	for _, message := range request.Messages {
		if message.Content == "" ||
			(message.Role != RoleSystem && message.Role != RoleUser && message.Role != RoleAssistant) {
			return &Error{Code: CodeInvalidRequest, Cause: errors.New("message role or content is invalid")}
		}
	}
	return nil
}

func wait(ctx, streamCtx context.Context, latency time.Duration) error {
	if latency <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-streamCtx.Done():
			return streamCtx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(latency)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-streamCtx.Done():
		return streamCtx.Err()
	case <-timer.C:
		return nil
	}
}
