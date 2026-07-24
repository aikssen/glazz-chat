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
	now = now.Add(time.Minute)
	gateway.streamErrors = nil
	gateway.stream = &scriptedStream{}
	stream, err := resilient.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
}

type scriptedGateway struct {
	streamErrors []error
	stream       Stream
	calls        int
}

func (gateway *scriptedGateway) Catalog(context.Context) ([]Model, error) {
	return nil, nil
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
