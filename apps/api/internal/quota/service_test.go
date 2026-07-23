package quota

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
)

func TestHashIPIsStableAndKeyed(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.10")
	first := HashIP([]byte("01234567890123456789012345678901"), ip)
	second := HashIP([]byte("01234567890123456789012345678901"), ip)
	other := HashIP([]byte("abcdefghijklmnopqrstuvwxyzABCDEF"), ip)
	if first != second {
		t.Fatal("same key and IP produced different hashes")
	}
	if first == other || first == ip.String() {
		t.Fatal("IP hash is not keyed")
	}
}

func TestMidnightUTCDefinesDailyReset(t *testing.T) {
	current := time.Date(2026, 7, 23, 23, 59, 0, 0, time.FixedZone("COT", -5*60*60))
	reset := midnightUTC(current).Add(24 * time.Hour)
	want := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	if !reset.Equal(want) {
		t.Fatalf("reset = %s, want %s", reset, want)
	}
}

type unavailableLimiter struct{}

func (unavailableLimiter) AddRateUsage(
	context.Context, string, int64, int64, time.Duration,
) (redisx.RateLimitResult, error) {
	return redisx.RateLimitResult{}, errors.New("unavailable")
}

func (unavailableLimiter) AcquireLease(
	context.Context, string, string, string, time.Duration,
) (bool, error) {
	return false, errors.New("unavailable")
}

func (unavailableLimiter) ReleaseLease(context.Context, string, string, string) (bool, error) {
	return false, errors.New("unavailable")
}

func TestReserveFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	service := New(
		nil,
		unavailableLimiter{},
		ids.NewFake(uuid.MustParse("018f3f4e-7b2a-7abc-8def-0123456789ab")),
		clock.NewFake(time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)),
		DefaultPolicy(),
		[]byte("01234567890123456789012345678901"),
	)
	_, err := service.Reserve(context.Background(), Actor{
		Type: Guest, ID: uuid.MustParse("018f3f4e-7b2a-7abc-8def-0123456789ac"),
	}, 100)
	if err == nil {
		t.Fatal("Reserve() succeeded while Redis was unavailable")
	}
}
