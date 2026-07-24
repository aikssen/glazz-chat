package provider

import (
	"context"
	"errors"
	"io"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type Request struct {
	Model           string
	Messages        []Message
	MaxOutputTokens int
	Temperature     float64
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
}

type FinishReason string

const (
	FinishStop   FinishReason = "stop"
	FinishLength FinishReason = "length"
	FinishSafety FinishReason = "safety"
	FinishError  FinishReason = "error"
)

type Chunk struct {
	Text          string
	Usage         *Usage
	FinishReason  FinishReason
	RequestID     string
	ProviderModel string
}

type Model struct {
	ID              string
	ContextWindow   int
	MaxOutputTokens int
	ChatCompletions bool
}

type Stream interface {
	Next(context.Context) (Chunk, error)
	Close() error
}

type Gateway interface {
	Catalog(context.Context) ([]Model, error)
	Stream(context.Context, Request) (Stream, error)
	Health(context.Context) error
}

type ErrorCode string

const (
	CodeInvalidRequest ErrorCode = "invalid_request"
	CodeUnauthorized   ErrorCode = "provider_unauthorized"
	CodeRateLimited    ErrorCode = "provider_rate_limited"
	CodeUnavailable    ErrorCode = "provider_unavailable"
	CodeTimeout        ErrorCode = "provider_timeout"
	CodeMalformed      ErrorCode = "provider_malformed_response"
)

type Error struct {
	Code       ErrorCode
	Retryable  bool
	StatusCode int
	Cause      error
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	return string(err.Code)
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func Normalize(err error) *Error {
	if err == nil {
		return nil
	}
	var providerError *Error
	if errors.As(err, &providerError) {
		return providerError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: CodeTimeout, Retryable: true, Cause: err}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: CodeUnavailable, Retryable: false, Cause: err}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return &Error{Code: CodeUnavailable, Retryable: true, Cause: err}
	}
	return &Error{Code: CodeUnavailable, Retryable: true, Cause: err}
}

type Options struct {
	RequestTimeout    time.Duration
	FirstChunkTimeout time.Duration
	IdleChunkTimeout  time.Duration
}

func DefaultOptions() Options {
	return Options{
		RequestTimeout: 90 * time.Second, FirstChunkTimeout: 15 * time.Second,
		IdleChunkTimeout: 30 * time.Second,
	}
}
