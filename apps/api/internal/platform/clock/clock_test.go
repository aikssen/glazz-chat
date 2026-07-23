package clock

import (
	"testing"
	"time"
)

func TestFakeClockIsDeterministic(t *testing.T) {
	start := time.Date(2026, 7, 23, 20, 0, 0, 0, time.FixedZone("offset", -5*60*60))
	clock := NewFake(start)
	if got := clock.Now(); !got.Equal(start.UTC()) || got.Location() != time.UTC {
		t.Fatalf("Now() = %v", got)
	}
	clock.Advance(time.Minute)
	if got := clock.Now(); !got.Equal(start.Add(time.Minute)) {
		t.Fatalf("Now() after advance = %v", got)
	}
}
