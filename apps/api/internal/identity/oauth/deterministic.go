package oauth

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

const deterministicCode = "glazz-e2e-approved"

type Deterministic struct {
	authorizeURL string
	profile      users.GoogleProfile
}

func NewDeterministic(cfg config.OAuth) (*Deterministic, error) {
	callback, err := url.Parse(cfg.CallbackURL)
	if err != nil || callback.Scheme == "" || callback.Host == "" || cfg.TestEmail == "" {
		return nil, errors.New("deterministic OAuth configuration is invalid")
	}
	callback.Path = "/api/v1/auth/test/authorize"
	callback.RawQuery = ""
	callback.Fragment = ""
	subject := strings.NewReplacer("@", "-", ".", "-").Replace(cfg.TestEmail)
	return &Deterministic{
		authorizeURL: callback.String(),
		profile: users.GoogleProfile{
			Subject:       "e2e-" + subject,
			Email:         cfg.TestEmail,
			EmailVerified: true,
			DisplayName:   "Glazz E2E Administrator",
		},
	}, nil
}

func (provider *Deterministic) AuthorizationURL(state, _, _ string) string {
	target, _ := url.Parse(provider.authorizeURL)
	query := target.Query()
	query.Set("state", state)
	target.RawQuery = query.Encode()
	return target.String()
}

func (provider *Deterministic) Exchange(
	_ context.Context,
	code, _, _ string,
) (users.GoogleProfile, error) {
	if code != deterministicCode {
		return users.GoogleProfile{}, errors.New("deterministic authorization code is invalid")
	}
	return provider.profile, nil
}
