package provider

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestResilientRetriesOnlyBeforeStreamStarts(t *testing.T) {
	gateway := &scriptedGateway{
		streamErrors: []error{
			&Error{Code: CodeUnavailable, Retryable: true},
			nil,
		},
		stream: &scriptedStream{
			chunks: []Chunk{{Text: "partial"}},
			err:    io.ErrUnexpectedEOF,
		},
	}
	resilient := NewResilient(gateway, ResilienceOptions{
		MaxConcurrent: 1, FailureLimit: 1, OpenDuration: time.Minute, PreStreamRetry: 1,
	})
	stream, err := resilient.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := stream.Next(context.Background())
	if err != nil || chunk.Text != "partial" {
		t.Fatalf("chunk = %#v, err = %v", chunk, err)
	}
	if _, err := stream.Next(context.Background()); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want disconnect", err)
	}
	if gateway.calls != 2 {
		t.Fatalf("gateway calls = %d, want two pre-stream attempts and no partial replay", gateway.calls)
	}
	if _, err := resilient.Stream(context.Background(), validRequest()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want open circuit", err)
	}
}

func TestResilientCircuitRecoversAfterOpenDuration(t *testing.T) {
	gateway := &scriptedGateway{
		streamErrors: []error{&Error{Code: CodeUnavailable, Retryable: true}},
	}
	resilient := NewResilient(gateway, ResilienceOptions{
		MaxConcurrent: 1, FailureLimit: 1, OpenDuration: time.Minute,
	})
	now := time.Date(2026, time.July, 23, 0, 0, 0, 0, time.UTC)
	resilient.now = func() time.Time { return now }
	if _, err := resilient.Stream(context.Background(), validRequest()); err == nil {
		t.Fatal("first stream error = nil")
	}
	if _, err := resilient.Stream(context.Background(), validRequest()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want open circuit", err)
	}
	if err := resilient.Health(context.Background()); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("health err = %v, want open circuit", err)
	}
	now = now.Add(time.Minute)
	gateway.streamErrors = nil
	gateway.stream = &scriptedStream{}
	if err := resilient.Health(context.Background()); err != nil {
		t.Fatalf("health after recovery = %v", err)
	}
	stream, err := resilient.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
}

func TestResilientBoundsConcurrentStreams(t *testing.T) {
	gateway := &scriptedGateway{stream: &scriptedStream{}}
	resilient := NewResilient(gateway, ResilienceOptions{
		MaxConcurrent: 1, FailureLimit: 2, OpenDuration: time.Minute,
	})
	first, err := resilient.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		stream Stream
		err    error
	}
	waiting := make(chan result, 1)
	go func() {
		stream, err := resilient.Stream(context.Background(), validRequest())
		waiting <- result{stream: stream, err: err}
	}()
	select {
	case <-waiting:
		t.Fatal("second stream bypassed the concurrency guard")
	case <-time.After(25 * time.Millisecond):
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-waiting:
		if result.err != nil {
			t.Fatal(result.err)
		}
		_ = result.stream.Close()
	case <-time.After(time.Second):
		t.Fatal("second stream did not start after capacity was released")
	}
}

type scriptedGateway struct {
	streamErrors []error
	stream       Stream
	calls        int
	healthErr    error
}

func (gateway *scriptedGateway) Catalog(context.Context) ([]Model, error) {
	return nil, nil
}

func (gateway *scriptedGateway) Health(context.Context) error {
	return gateway.healthErr
}

func (gateway *scriptedGateway) Stream(context.Context, Request) (Stream, error) {
	gateway.calls++
	if len(gateway.streamErrors) > 0 {
		err := gateway.streamErrors[0]
		gateway.streamErrors = gateway.streamErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	if gateway.stream == nil {
		gateway.stream = &scriptedStream{}
	}
	return gateway.stream, nil
}

type scriptedStream struct {
	chunks []Chunk
	err    error
	index  int
}

func (stream *scriptedStream) Next(context.Context) (Chunk, error) {
	if stream.index < len(stream.chunks) {
		chunk := stream.chunks[stream.index]
		stream.index++
		return chunk, nil
	}
	if stream.err != nil {
		return Chunk{}, stream.err
	}
	return Chunk{}, io.EOF
}

func (stream *scriptedStream) Close() error { return nil }

func validRequest() Request {
	return Request{
		Model: "model", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}
}
