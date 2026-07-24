package quota

import (
	"errors"
	"testing"
)

func TestAvailableOutputTokensUsesSmallestRemainingBudget(t *testing.T) {
	t.Parallel()

	got, err := availableOutputTokens(4096, 1994, 9000)
	if err != nil {
		t.Fatalf("available output tokens: %v", err)
	}
	if got != 1994 {
		t.Fatalf("expected remaining guest budget 1994, got %d", got)
	}
}

func TestAvailableOutputTokensRejectsExhaustedBudget(t *testing.T) {
	t.Parallel()

	_, err := availableOutputTokens(4096, 0, 9000)
	if !errors.Is(err, ErrExceeded) {
		t.Fatalf("expected ErrExceeded, got %v", err)
	}
}
