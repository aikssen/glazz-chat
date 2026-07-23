package tokens

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

func testConfig() config.Auth {
	return config.Auth{
		Issuer:         "https://api.glazz.test",
		Audience:       "glazz-web",
		ActiveKeyID:    "test-1",
		AccessTokenTTL: 15 * time.Minute,
	}
}

func TestSignAndVerify(t *testing.T) {
	timeSource := clock.NewFake(time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC))
	ring, err := NewEphemeral(testConfig(), timeSource)
	if err != nil {
		t.Fatalf("NewEphemeral() error = %v", err)
	}
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	sessionID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	raw, err := ring.Sign(userID, sessionID, 2)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	claims, err := ring.Verify(raw)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.Subject != userID.String() || claims.SessionID != sessionID.String() || claims.TokenVersion != 2 {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestVerifyRejectsTamperAndExpiry(t *testing.T) {
	timeSource := clock.NewFake(time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC))
	ring, _ := NewEphemeral(testConfig(), timeSource)
	raw, _ := ring.Sign(uuid.New(), uuid.New(), 1)

	parts := strings.Split(raw, ".")
	parts[2] = replacement(parts[2][:1]) + parts[2][1:]
	if _, err := ring.Verify(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered Verify() error = nil")
	}
	timeSource.Advance(16 * time.Minute)
	if _, err := ring.Verify(raw); err == nil {
		t.Fatal("expired Verify() error = nil")
	}
}

func TestVerifyRejectsWrongAudience(t *testing.T) {
	timeSource := clock.NewFake(time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC))
	signer, _ := NewEphemeral(testConfig(), timeSource)
	raw, _ := signer.Sign(uuid.New(), uuid.New(), 1)
	otherConfig := testConfig()
	otherConfig.Audience = "other-client"
	verifier, _ := NewEphemeral(otherConfig, timeSource)
	verifier.AddVerificationKey("test-1", signer.publicKeys["test-1"])
	if _, err := verifier.Verify(raw); err == nil {
		t.Fatal("wrong audience Verify() error = nil")
	}
}

func replacement(last string) string {
	if strings.EqualFold(last, "a") {
		return "b"
	}
	return "a"
}
