package ids

import (
	"testing"

	"github.com/google/uuid"
)

func TestFakeSourceReturnsConfiguredIDs(t *testing.T) {
	first := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	source := NewFake(first)
	got, err := source.New()
	if err != nil || got != first {
		t.Fatalf("New() = %v, %v", got, err)
	}
	if _, err := source.New(); err == nil {
		t.Fatal("exhausted New() error = nil")
	}
}

func TestUUIDv7(t *testing.T) {
	id, err := NewUUIDv7().New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if id.Version() != 7 {
		t.Fatalf("Version() = %d", id.Version())
	}
}
