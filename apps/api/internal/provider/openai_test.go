package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatibleStreamsAndNormalizesUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Error("authorization header missing")
		}
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(response, "data: {\"id\":\"req-1\",\"model\":\"provider-model\",\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n")
		_, _ = io.WriteString(response, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1}}\n\n")
		_, _ = io.WriteString(response, "data: [DONE]\n\n")
	}))
	defer server.Close()
	gateway, err := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := gateway.Stream(context.Background(), Request{
		Model: "model", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	chunk, err := stream.Next(context.Background())
	if err != nil || chunk.Text != "hello" || chunk.RequestID != "req-1" {
		t.Fatalf("chunk = %#v, err = %v", chunk, err)
	}
	terminal, err := stream.Next(context.Background())
	if err != nil || terminal.Usage == nil || terminal.Usage.InputTokens != 3 ||
		terminal.FinishReason != FinishStop {
		t.Fatalf("terminal = %#v, err = %v", terminal, err)
	}
}

func TestOpenAICompatibleTimesOutBeforeFirstChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		response.(http.Flusher).Flush()
		<-time.After(250 * time.Millisecond)
	}))
	defer server.Close()
	gateway, err := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{
		RequestTimeout: time.Second, FirstChunkTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := gateway.Stream(context.Background(), Request{
		Model: "model", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	_, err = stream.Next(context.Background())
	if normalized := Normalize(err); normalized.Code != CodeTimeout || !normalized.Retryable {
		t.Fatalf("err = %#v, want retryable timeout", err)
	}
}

func TestOpenAICompatibleTimesOutBetweenChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(response, "data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n")
		response.(http.Flusher).Flush()
		<-time.After(250 * time.Millisecond)
	}))
	defer server.Close()
	gateway, err := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{
		RequestTimeout: time.Second, FirstChunkTimeout: 100 * time.Millisecond,
		IdleChunkTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := gateway.Stream(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if chunk, err := stream.Next(context.Background()); err != nil || chunk.Text != "first" {
		t.Fatalf("first chunk = %#v, err = %v", chunk, err)
	}
	if _, err := stream.Next(context.Background()); Normalize(err).Code != CodeTimeout {
		t.Fatalf("idle err = %#v, want timeout", err)
	}
}

func TestOpenAICompatibleHealthUsesAuthenticatedCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("health request = %s, authorization %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"data":[{"id":"deepseek-v4-flash"}]}`)
	}))
	defer server.Close()
	gateway, err := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gateway.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAICompatibleNormalizesDisconnectAfterPartialOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(response, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n")
	}))
	defer server.Close()
	gateway, _ := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{})
	stream, err := gateway.Stream(context.Background(), Request{
		Model: "model", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	chunk, err := stream.Next(context.Background())
	if err != nil || chunk.Text != "partial" {
		t.Fatalf("chunk = %#v, err = %v", chunk, err)
	}
	_, err = stream.Next(context.Background())
	if normalized := Normalize(err); normalized.Code != CodeUnavailable || !normalized.Retryable {
		t.Fatalf("err = %#v, want retryable disconnect", err)
	}
}

func TestOpenAICompatibleRejectsMalformedStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(response, "data: not-json\n\n")
	}))
	defer server.Close()
	gateway, _ := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{})
	stream, err := gateway.Stream(context.Background(), Request{
		Model: "model", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	_, err = stream.Next(context.Background())
	if err == nil || Normalize(err).Code != CodeMalformed {
		t.Fatalf("err = %v, want malformed", err)
	}
}

func TestOpenAICompatibleNormalizesRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	gateway, _ := NewOpenAICompatible(server.URL, "secret", server.Client(), Options{})
	_, err := gateway.Stream(context.Background(), Request{
		Model: "model", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	var providerError *Error
	if !errors.As(err, &providerError) || providerError.Code != CodeRateLimited || !providerError.Retryable {
		t.Fatalf("err = %#v, want retryable rate limit", err)
	}
}
