package settings

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type cacheStub struct {
	value       string
	getErr      error
	deleteErr   error
	deleteCalls int
}

func (stub *cacheStub) Get(context.Context, string, string) (string, error) {
	return stub.value, stub.getErr
}

func (stub *cacheStub) Put(context.Context, string, string, string, time.Duration) error {
	return nil
}

func (stub *cacheStub) Delete(context.Context, string, string) error {
	stub.deleteCalls++
	return stub.deleteErr
}

func TestLoadUsesTypedCachedSnapshot(t *testing.T) {
	t.Parallel()
	expected := Snapshot{
		Maintenance: true, GuestMessageLimit: 4, GuestOutputTokenLimit: 2000,
		UserMessageLimit: 50, UserOutputTokenLimit: 50000,
		GlobalOutputTokenLimit: 10000000, GlobalConcurrentStreams: 100,
		SystemPrompt: "policy", SummaryModelID: "00000000-0000-7000-8000-000000000101",
		InputSafetyCategories: []string{"input"}, OutputSafetyCategories: []string{"output"},
	}
	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	cache := &cacheStub{value: string(encoded)}
	actual, err := New(nil, cache).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if actual.SystemPrompt != expected.SystemPrompt ||
		actual.GlobalOutputTokenLimit != expected.GlobalOutputTokenLimit ||
		len(actual.OutputSafetyCategories) != 1 {
		t.Fatalf("cached snapshot = %#v", actual)
	}
}

func TestInvalidateDeletesSharedSnapshot(t *testing.T) {
	t.Parallel()
	cache := &cacheStub{}
	if err := New(nil, cache).Invalidate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", cache.deleteCalls)
	}

	expected := errors.New("cache unavailable")
	cache.deleteErr = expected
	if err := New(nil, cache).Invalidate(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("Invalidate() error = %v", err)
	}
}
