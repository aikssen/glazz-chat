package browser

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/tokens"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

const (
	AccessCookie  = "glazz_access"
	RefreshCookie = "glazz_refresh"
	CSRFCookie    = "glazz_csrf"
)

type actorKey string

const currentActorKey actorKey = "current_actor"

var ErrUnauthenticated = errors.New("browser authentication is invalid")

type Actor struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

type Manager struct {
	config     config.Cookies
	accessTTL  time.Duration
	refreshTTL time.Duration
	key        []byte
}

func New(cfg config.Cookies, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{config: cfg, accessTTL: accessTTL, refreshTTL: refreshTTL, key: cfg.SigningKey}
}

func (manager *Manager) Issue(
	response http.ResponseWriter,
	credentials sessions.Credentials,
) (string, error) {
	csrf, err := ids.SecureToken(32)
	if err != nil {
		return "", err
	}
	signedCSRF := manager.sign(csrf)
	manager.set(response, AccessCookie, credentials.AccessToken, manager.accessTTL, true)
	manager.set(response, RefreshCookie, credentials.RefreshToken, manager.refreshTTL, true)
	manager.set(response, CSRFCookie, signedCSRF, manager.refreshTTL, false)
	return signedCSRF, nil
}

func (manager *Manager) Clear(response http.ResponseWriter) {
	for _, cookie := range []struct {
		name     string
		httpOnly bool
	}{
		{name: AccessCookie, httpOnly: true},
		{name: RefreshCookie, httpOnly: true},
		{name: CSRFCookie, httpOnly: false},
	} {
		http.SetCookie(response, &http.Cookie{
			Name:     cookie.name,
			Value:    "",
			Path:     "/",
			Domain:   manager.config.Domain,
			MaxAge:   -1,
			HttpOnly: cookie.httpOnly,
			Secure:   manager.config.Secure,
			SameSite: manager.sameSite(),
		})
	}
}

func (manager *Manager) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet || request.Method == http.MethodHead ||
			request.Method == http.MethodOptions {
			next.ServeHTTP(response, request)
			return
		}
		cookie, err := request.Cookie(CSRFCookie)
		header := request.Header.Get("X-CSRF-Token")
		if err != nil || header == "" ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 ||
			!manager.verify(header) {
			httpx.WriteError(response, request, http.StatusForbidden, "forbidden", "CSRF validation failed.")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (manager *Manager) RefreshCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie(CSRFCookie)
		header := request.Header.Get("X-CSRF-Token")
		if err == nil && header != "" &&
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) == 1 &&
			!manager.verify(header) {
			manager.Clear(response)
			httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Session cookies expired.")
			return
		}
		manager.CSRF(next).ServeHTTP(response, request)
	})
}

func Authenticate(
	ring *tokens.KeyRing,
	sessionService *sessions.Service,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			actor, err := Resolve(request, ring, sessionService)
			if err != nil {
				httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Session is no longer active.")
				return
			}
			ctx := context.WithValue(request.Context(), currentActorKey, actor)
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

func Resolve(
	request *http.Request,
	ring *tokens.KeyRing,
	sessionService *sessions.Service,
) (Actor, error) {
	cookie, err := request.Cookie(AccessCookie)
	if err != nil || cookie.Value == "" {
		return Actor{}, ErrUnauthenticated
	}
	claims, err := ring.Verify(cookie.Value)
	if err != nil {
		return Actor{}, ErrUnauthenticated
	}
	userID, sessionID, err := sessionService.ValidateAccessSession(request.Context(), claims)
	if err != nil {
		return Actor{}, ErrUnauthenticated
	}
	return Actor{UserID: userID, SessionID: sessionID}, nil
}

func CurrentActor(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(currentActorKey).(Actor)
	return actor, ok
}

func RefreshToken(request *http.Request) (string, error) {
	cookie, err := request.Cookie(RefreshCookie)
	if err != nil || cookie.Value == "" {
		return "", errors.New("refresh cookie is missing")
	}
	return cookie.Value, nil
}

func (manager *Manager) set(
	response http.ResponseWriter,
	name, value string,
	lifetime time.Duration,
	httpOnly bool,
) {
	http.SetCookie(response, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   manager.config.Domain,
		MaxAge:   int(lifetime.Seconds()),
		HttpOnly: httpOnly,
		Secure:   manager.config.Secure,
		SameSite: manager.sameSite(),
	})
}

func (manager *Manager) sign(value string) string {
	mac := hmac.New(sha256.New, manager.key)
	_, _ = mac.Write([]byte(value))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return value + "." + signature
}

func (manager *Manager) verify(value string) bool {
	token, _, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	expected := manager.sign(token)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(value)) == 1
}

func (manager *Manager) sameSite() http.SameSite {
	if manager.config.SameSite == "strict" {
		return http.SameSiteStrictMode
	}
	return http.SameSiteLaxMode
}
