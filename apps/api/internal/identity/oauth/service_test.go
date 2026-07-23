package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
)

type memoryStates struct {
	values map[string]string
}

func (states *memoryStates) Put(_ context.Context, namespace, id, value string, _ time.Duration) error {
	states.values[namespace+":"+id] = value
	return nil
}

func (states *memoryStates) Take(_ context.Context, namespace, id string) (string, error) {
	key := namespace + ":" + id
	value, ok := states.values[key]
	if !ok {
		return "", errors.New("missing")
	}
	delete(states.values, key)
	return value, nil
}

type fakeProvider struct{}

func (fakeProvider) AuthorizationURL(state, nonce, challenge string) string {
	return "https://accounts.example/authorize?state=" + state + "&nonce=" + nonce + "&challenge=" + challenge
}

func (fakeProvider) Exchange(context.Context, string, string, string) (users.GoogleProfile, error) {
	return users.GoogleProfile{}, nil
}

func TestStartUsesSingleUseServerStateAndPKCE(t *testing.T) {
	states := &memoryStates{values: map[string]string{}}
	service := New(states, fakeProvider{}, nil, nil, 10*time.Minute)

	authorizationURL, err := service.Start(context.Background(), StartInput{
		ReturnTo: "/", TermsAccepted: true, PrivacyAccepted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authorizationURL, "state=") ||
		!strings.Contains(authorizationURL, "nonce=") ||
		!strings.Contains(authorizationURL, "challenge=") {
		t.Fatalf("authorization URL does not contain state protections: %s", authorizationURL)
	}
	if len(states.values) != 1 {
		t.Fatalf("stored states = %d, want 1", len(states.values))
	}
}

func TestStartRejectsUnsafeReturnPathsAndMissingConsent(t *testing.T) {
	service := New(&memoryStates{values: map[string]string{}}, fakeProvider{}, nil, nil, time.Minute)
	for _, returnTo := range []string{"https://evil.example", "//evil.example", "/ok\r\nLocation: bad", `\evil`} {
		_, err := service.Start(context.Background(), StartInput{
			ReturnTo: returnTo, TermsAccepted: true, PrivacyAccepted: true,
		})
		if !errors.Is(err, ErrInvalidReturn) {
			t.Fatalf("Start(%q) error = %v, want ErrInvalidReturn", returnTo, err)
		}
	}
	_, err := service.Start(context.Background(), StartInput{ReturnTo: "/"})
	if !errors.Is(err, ErrConsentMissing) {
		t.Fatalf("missing consent error = %v, want ErrConsentMissing", err)
	}
}
