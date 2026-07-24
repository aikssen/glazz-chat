package guests

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

const (
	CookieName     = "glazz_guest"
	CSRFCookieName = "glazz_guest_csrf"
)

var ErrGuestUnauthenticated = errors.New("guest session is missing, expired, or migrated")

type Service struct {
	database *database.Pool
	ids      ids.Source
	clock    clock.Clock
	cookies  config.Cookies
	lifetime time.Duration
}

type Allowance struct {
	MessagesUsed     int32     `json:"messagesUsed"`
	MessageLimit     int32     `json:"messageLimit"`
	OutputTokensUsed int32     `json:"outputTokensUsed"`
	OutputTokenLimit int32     `json:"outputTokenLimit"`
	Exhausted        bool      `json:"exhausted"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

func New(
	pool *database.Pool,
	idSource ids.Source,
	timeSource clock.Clock,
	cookies config.Cookies,
	lifetime time.Duration,
) *Service {
	return &Service{
		database: pool, ids: idSource, clock: timeSource, cookies: cookies, lifetime: lifetime,
	}
}

func (service *Service) CreateOrResume(
	ctx context.Context,
	request *http.Request,
	response http.ResponseWriter,
) (Allowance, bool, error) {
	if record, err := service.fromRequest(ctx, request); err == nil {
		_ = service.database.Queries().TouchGuestSession(ctx, store.TouchGuestSessionParams{
			ID: record.ID, LastSeenAt: timestamp(service.clock.Now()),
		})
		if err := service.issueCSRF(response); err != nil {
			return Allowance{}, false, err
		}
		return allowance(record), false, nil
	}

	rawToken, err := ids.SecureToken(32)
	if err != nil {
		return Allowance{}, false, err
	}
	id, err := service.ids.New()
	if err != nil {
		return Allowance{}, false, err
	}
	tokenHash := sha256.Sum256([]byte(rawToken))
	expiresAt := service.clock.Now().Add(service.lifetime)
	record, err := service.database.Queries().CreateGuestSession(ctx, store.CreateGuestSessionParams{
		ID: id, IdentityHash: tokenHash[:], ExpiresAt: timestamp(expiresAt),
	})
	if err != nil {
		return Allowance{}, false, fmt.Errorf("create guest session: %w", err)
	}
	http.SetCookie(response, &http.Cookie{
		Name:     CookieName,
		Value:    service.sign(rawToken),
		Path:     "/",
		Domain:   service.cookies.Domain,
		MaxAge:   int(service.lifetime.Seconds()),
		HttpOnly: true,
		Secure:   service.cookies.Secure,
		SameSite: service.sameSite(),
	})
	if err := service.issueCSRF(response); err != nil {
		return Allowance{}, false, err
	}
	return allowance(record), true, nil
}

func (service *Service) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet || request.Method == http.MethodHead ||
			request.Method == http.MethodOptions {
			next.ServeHTTP(response, request)
			return
		}
		cookie, err := request.Cookie(CSRFCookieName)
		header := request.Header.Get("X-CSRF-Token")
		_, valid := service.verify(header)
		if err != nil || header == "" ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 || !valid {
			httpx.WriteError(response, request, http.StatusForbidden, "forbidden", "CSRF validation failed.")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (service *Service) Current(ctx context.Context, request *http.Request) (Allowance, error) {
	record, err := service.fromRequest(ctx, request)
	if err != nil {
		return Allowance{}, err
	}
	return allowance(record), nil
}

func (service *Service) ID(ctx context.Context, request *http.Request) (*uuid.UUID, error) {
	record, err := service.fromRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	return &record.ID, nil
}

func (service *Service) fromRequest(
	ctx context.Context,
	request *http.Request,
) (store.GuestSession, error) {
	cookie, err := request.Cookie(CookieName)
	if err != nil {
		return store.GuestSession{}, ErrGuestUnauthenticated
	}
	rawToken, ok := service.verify(cookie.Value)
	if !ok {
		return store.GuestSession{}, ErrGuestUnauthenticated
	}
	tokenHash := sha256.Sum256([]byte(rawToken))
	record, err := service.database.Queries().GetGuestSessionByIdentityHash(ctx, tokenHash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return store.GuestSession{}, ErrGuestUnauthenticated
	}
	if err != nil {
		return store.GuestSession{}, fmt.Errorf("get guest session: %w", err)
	}
	if !record.ExpiresAt.Valid || !record.ExpiresAt.Time.After(service.clock.Now()) ||
		record.MigratedUserID != nil {
		return store.GuestSession{}, ErrGuestUnauthenticated
	}
	return record, nil
}

func (service *Service) sign(rawToken string) string {
	mac := hmac.New(sha256.New, service.cookies.SigningKey)
	_, _ = mac.Write([]byte(rawToken))
	return rawToken + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (service *Service) verify(value string) (string, bool) {
	token, _, ok := strings.Cut(value, ".")
	if !ok || token == "" {
		return "", false
	}
	expected := service.sign(token)
	return token, subtle.ConstantTimeCompare([]byte(expected), []byte(value)) == 1
}

func (service *Service) sameSite() http.SameSite {
	if service.cookies.SameSite == "strict" {
		return http.SameSiteStrictMode
	}
	return http.SameSiteLaxMode
}

func (service *Service) issueCSRF(response http.ResponseWriter) error {
	raw, err := ids.SecureToken(32)
	if err != nil {
		return err
	}
	http.SetCookie(response, &http.Cookie{
		Name: CSRFCookieName, Value: service.sign(raw), Path: "/",
		Domain: service.cookies.Domain, MaxAge: int(service.lifetime.Seconds()),
		HttpOnly: false, Secure: service.cookies.Secure, SameSite: service.sameSite(),
	})
	return nil
}

func allowance(record store.GuestSession) Allowance {
	return Allowance{
		MessagesUsed:     record.PromptCount,
		MessageLimit:     4,
		OutputTokensUsed: record.OutputTokenCount,
		OutputTokenLimit: 2000,
		Exhausted:        record.PromptCount >= 4 || record.OutputTokenCount >= 2000,
		ExpiresAt:        record.ExpiresAt.Time.UTC(),
	}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
