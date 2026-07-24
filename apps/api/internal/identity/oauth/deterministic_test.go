package oauth

import (
	"context"
	"net/url"
	"testing"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

func TestDeterministicProviderUsesCallbackOriginAndFixedProfile(t *testing.T) {
	provider, err := NewDeterministic(config.OAuth{
		CallbackURL: "http://localhost:8080/api/v1/auth/google/callback",
		TestEmail:   "e2e-admin@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(provider.AuthorizationURL("state-value", "nonce", "challenge"))
	if err != nil {
		t.Fatal(err)
	}
	if target.Path != "/api/v1/auth/test/authorize" || target.Query().Get("state") != "state-value" {
		t.Fatalf("authorization URL = %s", target)
	}
	profile, err := provider.Exchange(context.Background(), deterministicCode, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Email != "e2e-admin@example.com" || !profile.EmailVerified {
		t.Fatalf("profile = %+v", profile)
	}
}
