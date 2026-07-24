package provider

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestFakeStreamsDeterministically(t *testing.T) {
	fake := NewFake(FakeOptions{Chunks: []string{"a", "b"}, Usage: Usage{OutputTokens: 2}})
	stream, err := fake.Stream(context.Background(), Request{
		Model: "deepseek-v4-flash", MaxOutputTokens: 20,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	first, err := stream.Next(context.Background())
	if err != nil || first.Text != "a" {
		t.Fatalf("first chunk = %#v, err = %v", first, err)
	}
	second, err := stream.Next(context.Background())
	if err != nil || second.Text != "b" {
		t.Fatalf("second chunk = %#v, err = %v", second, err)
	}
	terminal, err := stream.Next(context.Background())
	if err != nil || terminal.Usage == nil || terminal.FinishReason != FinishStop {
		t.Fatalf("terminal = %#v, err = %v", terminal, err)
	}
	if _, err := stream.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("final err = %v, want EOF", err)
	}
}

func TestFakePartialFailureIsNotRetryable(t *testing.T) {
	fake := NewFake(FakeOptions{Chunks: []string{"a", "b"}, FailAfter: 1})
	stream, err := fake.Stream(context.Background(), Request{
		Model: "model", MaxOutputTokens: 2,
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Next(context.Background()); err == nil || Normalize(err).Retryable {
		t.Fatalf("partial failure = %v, want non-retryable", err)
	}
}
