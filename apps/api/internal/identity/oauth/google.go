package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

type Google struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewGoogle(ctx context.Context, cfg config.OAuth) (*Google, error) {
	if !cfg.Enabled {
		return nil, errors.New("Google OAuth is disabled")
	}
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("discover Google OpenID provider: %w", err)
	}
	return &Google{
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (google *Google) AuthorizationURL(state, nonce, challenge string) string {
	return google.oauth.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

func (google *Google) Exchange(
	ctx context.Context,
	code, verifier, expectedNonce string,
) (users.GoogleProfile, error) {
	token, err := google.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return users.GoogleProfile{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return users.GoogleProfile{}, errors.New("Google response omitted ID token")
	}
	idToken, err := google.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return users.GoogleProfile{}, fmt.Errorf("verify Google ID token: %w", err)
	}
	var claims struct {
		Nonce         string `json:"nonce"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return users.GoogleProfile{}, fmt.Errorf("decode Google ID token claims: %w", err)
	}
	if claims.Nonce == "" || claims.Nonce != expectedNonce {
		return users.GoogleProfile{}, errors.New("Google ID token nonce does not match")
	}
	return users.GoogleProfile{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		DisplayName:   claims.Name,
		AvatarURL:     claims.Picture,
	}, nil
}
