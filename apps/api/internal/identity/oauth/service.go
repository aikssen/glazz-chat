package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

const stateNamespace = "oauth_state"

var (
	ErrInvalidState     = errors.New("OAuth state is invalid, expired, or already used")
	ErrInvalidReturn    = errors.New("OAuth return path is not allowed")
	ErrConsentMissing   = errors.New("terms and privacy acceptance are required")
	ErrIdentityMismatch = errors.New("reauthentication identity does not match")
)

type StateStore interface {
	Put(context.Context, string, string, string, time.Duration) error
	Take(context.Context, string, string) (string, error)
}

type Provider interface {
	AuthorizationURL(state, nonce, challenge string) string
	Exchange(context.Context, string, string, string) (users.GoogleProfile, error)
}

type stateRecord struct {
	Verifier        string     `json:"verifier"`
	Nonce           string     `json:"nonce"`
	ReturnTo        string     `json:"returnTo"`
	TermsAccepted   bool       `json:"termsAccepted"`
	PrivacyAccepted bool       `json:"privacyAccepted"`
	GuestID         *uuid.UUID `json:"guestId,omitempty"`
	Locale          string     `json:"locale"`
	ExpectedUserID  *uuid.UUID `json:"expectedUserId,omitempty"`
}

type StartInput struct {
	ReturnTo        string
	TermsAccepted   bool
	PrivacyAccepted bool
	GuestID         *uuid.UUID
	Locale          string
	ExpectedUserID  *uuid.UUID
}

type Completion struct {
	User        store.User
	Credentials sessions.Credentials
	ReturnTo    string
	Created     bool
}

type Service struct {
	states   StateStore
	provider Provider
	users    *users.Service
	sessions *sessions.Service
	stateTTL time.Duration
}

func New(
	states StateStore,
	provider Provider,
	userService *users.Service,
	sessionService *sessions.Service,
	stateTTL time.Duration,
) *Service {
	return &Service{
		states: states, provider: provider, users: userService, sessions: sessionService, stateTTL: stateTTL,
	}
}

func (service *Service) Start(ctx context.Context, input StartInput) (string, error) {
	if !input.TermsAccepted || !input.PrivacyAccepted {
		return "", ErrConsentMissing
	}
	returnTo, err := safeReturnPath(input.ReturnTo)
	if err != nil {
		return "", err
	}
	state, err := ids.SecureToken(32)
	if err != nil {
		return "", err
	}
	verifier, err := ids.SecureToken(48)
	if err != nil {
		return "", err
	}
	nonce, err := ids.SecureToken(32)
	if err != nil {
		return "", err
	}
	if input.Locale != "es" {
		input.Locale = "en"
	}
	record, err := json.Marshal(stateRecord{
		Verifier:        verifier,
		Nonce:           nonce,
		ReturnTo:        returnTo,
		TermsAccepted:   input.TermsAccepted,
		PrivacyAccepted: input.PrivacyAccepted,
		GuestID:         input.GuestID,
		Locale:          input.Locale,
		ExpectedUserID:  input.ExpectedUserID,
	})
	if err != nil {
		return "", fmt.Errorf("encode OAuth state: %w", err)
	}
	if err := service.states.Put(ctx, stateNamespace, state, string(record), service.stateTTL); err != nil {
		return "", fmt.Errorf("store OAuth state: %w", err)
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return service.provider.AuthorizationURL(state, nonce, challenge), nil
}

func (service *Service) Complete(
	ctx context.Context,
	state, code, deviceLabel, requestID string,
	ipHash []byte,
) (Completion, error) {
	if state == "" || code == "" {
		return Completion{}, ErrInvalidState
	}
	record, err := service.takeState(ctx, state)
	if err != nil {
		return Completion{}, ErrInvalidState
	}
	profile, err := service.provider.Exchange(ctx, code, record.Verifier, record.Nonce)
	if err != nil {
		return Completion{}, fmt.Errorf("exchange Google authorization: %w", err)
	}
	if record.ExpectedUserID != nil {
		current, lookupErr := service.users.FindByGoogleSubject(ctx, profile.Subject)
		if lookupErr != nil || current.ID != *record.ExpectedUserID {
			return Completion{}, ErrIdentityMismatch
		}
	}
	user, created, err := service.users.ProvisionGoogle(ctx, users.ProvisionInput{
		Profile:         profile,
		Locale:          record.Locale,
		TermsAccepted:   record.TermsAccepted,
		PrivacyAccepted: record.PrivacyAccepted,
		IPHash:          ipHash,
		GuestID:         record.GuestID,
		RequestID:       requestID,
	})
	if err != nil {
		return Completion{}, err
	}
	credentials, err := service.sessions.Create(ctx, user.ID, deviceLabel)
	if err != nil {
		return Completion{}, fmt.Errorf("create browser session: %w", err)
	}
	return Completion{
		User: user, Credentials: credentials, ReturnTo: record.ReturnTo, Created: created,
	}, nil
}

func (service *Service) Cancel(ctx context.Context, state string) (string, error) {
	record, err := service.takeState(ctx, state)
	if err != nil {
		return "", err
	}
	return record.ReturnTo, nil
}

func (service *Service) takeState(ctx context.Context, state string) (stateRecord, error) {
	if state == "" {
		return stateRecord{}, ErrInvalidState
	}
	raw, err := service.states.Take(ctx, stateNamespace, state)
	if err != nil {
		return stateRecord{}, ErrInvalidState
	}
	var record stateRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil ||
		record.Verifier == "" || record.Nonce == "" || record.ReturnTo == "" {
		return stateRecord{}, ErrInvalidState
	}
	return record, nil
}

func safeReturnPath(raw string) (string, error) {
	if raw == "" {
		return "/", nil
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.ContainsAny(raw, "\r\n\\") {
		return "", ErrInvalidReturn
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		return "", ErrInvalidReturn
	}
	return parsed.RequestURI(), nil
}
