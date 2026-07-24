package server

import (
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/admin"
	"github.com/aikssen/glazz-chat/apps/api/internal/chat"
	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/guests"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	identityoauth "github.com/aikssen/glazz-chat/apps/api/internal/identity/oauth"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/telemetry"
	"github.com/aikssen/glazz-chat/apps/api/internal/privacy"
	"github.com/aikssen/glazz-chat/apps/api/internal/quota"
	"github.com/aikssen/glazz-chat/apps/api/internal/realtime"
	"github.com/aikssen/glazz-chat/apps/api/internal/settings"
)

type Dependencies struct {
	Config      config.Config
	Database    *database.Pool
	Redis       *redisx.Client
	Guests      *guests.Service
	OAuth       *identityoauth.Service
	Sessions    *sessions.Service
	Browser     *browser.Manager
	Auth        func(http.Handler) http.Handler
	Telemetry   *telemetry.Runtime
	Logger      *slog.Logger
	IDs         ids.Source
	Models      *models.Service
	Chats       *conversations.Service
	ResolveUser func(*http.Request) (browser.Actor, error)
	Tickets     *realtime.Tickets
	Realtime    *realtime.Handler
	ChatEngine  *chat.Service
	Admin       *admin.Service
	Privacy     *privacy.Service
	Settings    *settings.Service
}

func Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(httpx.SecurityHeaders)
	router.Get("/api/v1/health/live", liveness)
	return router
}

func New(deps Dependencies) http.Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	router := chi.NewRouter()
	router.Use(httpx.RequestIDs(deps.IDs))
	router.Use(httpx.Recovery(deps.Logger))
	router.Use(httpx.SecurityHeaders)
	router.Use(httpx.ClientIPs(deps.Config.Runtime.TrustedProxies))
	router.Use(httpx.CORS(deps.Config.Runtime.AllowedOrigins))
	router.Use(httpx.MaxBody(deps.Config.Runtime.MaxBodyBytes))
	router.Use(httpx.Timeout(deps.Config.Runtime.RequestTimeout))
	if deps.Telemetry != nil {
		router.Use(deps.Telemetry.Middleware(deps.Logger))
		router.Handle(deps.Config.Telemetry.MetricsPath, deps.Telemetry.MetricsHandler())
	}
	router.NotFound(httpx.NotFound)
	router.MethodNotAllowed(httpx.MethodNotAllowed)

	router.Route("/api/v1", func(router chi.Router) {
		router.Get("/health/live", liveness)
		router.Get("/health/ready", deps.readiness)
		router.Get("/config/public", deps.publicConfig)
		router.Post("/guest-sessions", deps.createOrResumeGuest)
		router.Get("/guest-sessions/current", deps.currentGuest)
		router.Get("/models", deps.listModels)
		router.Get("/conversations", deps.listConversations)
		router.Post("/conversations", deps.withActorCSRF(deps.createConversation))
		router.Get("/conversations/{conversationId}", deps.getConversation)
		router.Patch("/conversations/{conversationId}", deps.withActorCSRF(deps.updateConversation))
		router.Delete("/conversations/{conversationId}", deps.withActorCSRF(deps.deleteConversation))
		router.Get("/conversations/{conversationId}/messages", deps.listConversationMessages)
		router.Post("/conversations/{conversationId}/retry", deps.withActorCSRF(deps.retryGeneration))
		router.Get("/usage", deps.usage)
		router.Post("/auth/ws-ticket", deps.withActorCSRF(deps.createWebSocketTicket))
		router.Get("/ws", deps.websocket)
		router.Get("/auth/google/start", deps.startGoogle)
		router.Get("/auth/google/callback", deps.completeGoogle)
		if deps.Config.OAuth.TestMode {
			router.Get("/auth/test/authorize", deps.testAuthorize)
		}

		router.With(deps.Browser.RefreshCSRF).Post("/auth/refresh", deps.refresh)
		router.Group(func(protected chi.Router) {
			protected.Use(deps.Auth)
			protected.Get("/me", deps.me)
			protected.Get("/me/sessions", deps.listSessions)
			protected.With(deps.Browser.CSRF).Post("/me/reauthenticate", deps.startReauthentication)
			protected.With(deps.Browser.CSRF).Post("/auth/logout", deps.logout)
			protected.With(deps.Browser.CSRF).Delete("/me/sessions/{sessionId}", deps.revokeSession)
			protected.Get("/me/deletion", deps.getAccountDeletion)
			protected.With(deps.Browser.CSRF, deps.requireRecent).Delete("/me", deps.requestAccountDeletion)

			protected.Route("/admin", func(adminRouter chi.Router) {
				adminRouter.Use(deps.requireAdmin)
				adminRouter.Get("/models", deps.adminListModels)
				adminRouter.With(deps.Browser.CSRF).Post("/models/sync", deps.adminSyncModels)
				adminRouter.With(deps.Browser.CSRF, deps.requireRecent).Patch("/models/{modelId}", deps.adminUpdateModel)
				adminRouter.Get("/settings", deps.adminListSettings)
				adminRouter.With(deps.Browser.CSRF, deps.requireRecent).Patch("/settings/{key}", deps.adminUpdateSetting)
				adminRouter.Get("/users", deps.adminListUsers)
				adminRouter.With(deps.Browser.CSRF, deps.requireRecent).Patch("/users/{userId}/role", deps.adminUpdateUserRole)
				adminRouter.Get("/usage", deps.adminUsage)
				adminRouter.Get("/audit-log", deps.adminAudit)
			})
		})
	})
	return router
}

func liveness(response http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (deps Dependencies) readiness(response http.ResponseWriter, request *http.Request) {
	status := map[string]string{"postgres": "up", "redis": "up"}
	code := http.StatusOK
	if err := deps.Database.Ping(request.Context()); err != nil {
		status["postgres"] = "down"
		code = http.StatusServiceUnavailable
	}
	if err := deps.Redis.Ping(request.Context()); err != nil {
		status["redis"] = "down"
		code = http.StatusServiceUnavailable
	}
	state := "ready"
	if code != http.StatusOK {
		state = "not_ready"
	}
	httpx.WriteJSON(response, code, map[string]any{"status": state, "dependencies": status})
}

func (deps Dependencies) publicConfig(response http.ResponseWriter, request *http.Request) {
	maintenance := deps.Config.Runtime.Maintenance
	messageLimit := int64(4)
	outputTokenLimit := int64(2000)
	if deps.Settings != nil {
		snapshot, err := deps.Settings.Load(request.Context())
		if err != nil {
			deps.internalError(response, request, err)
			return
		}
		maintenance = maintenance || snapshot.Maintenance
		messageLimit = snapshot.GuestMessageLimit
		outputTokenLimit = snapshot.GuestOutputTokenLimit
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]any{
		"locales":     []string{"en", "es"},
		"maintenance": maintenance,
		"legal": map[string]any{
			"termsVersion": deps.Config.Auth.TermsVersion, "privacyVersion": deps.Config.Auth.PrivacyVersion,
			"minimumAge": 18,
		},
		"guestPolicy": map[string]any{
			"messageLimit": messageLimit, "outputTokenLimit": outputTokenLimit, "resetsAutomatically": false,
		},
	})
}

func (deps Dependencies) createOrResumeGuest(response http.ResponseWriter, request *http.Request) {
	maintenance := deps.Config.Runtime.Maintenance
	if deps.Settings != nil {
		snapshot, err := deps.Settings.Load(request.Context())
		if err != nil {
			deps.internalError(response, request, err)
			return
		}
		maintenance = maintenance || snapshot.Maintenance
	}
	if maintenance {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "maintenance", "Service is temporarily unavailable.")
		return
	}
	allowance, created, err := deps.Guests.CreateOrResume(request.Context(), request, response)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(response, status, allowance)
}

func (deps Dependencies) currentGuest(response http.ResponseWriter, request *http.Request) {
	allowance, err := deps.Guests.Current(request.Context(), request)
	if errors.Is(err, guests.ErrGuestUnauthenticated) {
		httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Guest session is not active.")
		return
	}
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, allowance)
}

func (deps Dependencies) startGoogle(response http.ResponseWriter, request *http.Request) {
	if deps.OAuth == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Google login is not configured.")
		return
	}
	terms, termsErr := strconv.ParseBool(request.URL.Query().Get("termsAccepted"))
	privacy, privacyErr := strconv.ParseBool(request.URL.Query().Get("privacyAccepted"))
	if termsErr != nil || privacyErr != nil || !terms || !privacy {
		httpx.WriteError(response, request, http.StatusBadRequest, "consent_required", "Current terms and privacy acceptance are required.")
		return
	}
	guestID, _ := deps.Guests.ID(request.Context(), request)
	authorizationURL, err := deps.OAuth.Start(request.Context(), identityoauth.StartInput{
		ReturnTo: request.URL.Query().Get("returnTo"), TermsAccepted: terms,
		PrivacyAccepted: privacy, GuestID: guestID, Locale: locale(request),
	})
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Login request is invalid.")
		return
	}
	http.Redirect(response, request, authorizationURL, http.StatusFound)
}

func (deps Dependencies) completeGoogle(response http.ResponseWriter, request *http.Request) {
	if deps.OAuth == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Google login is not configured.")
		return
	}
	if request.URL.Query().Get("error") == "access_denied" {
		returnTo, err := deps.OAuth.Cancel(request.Context(), request.URL.Query().Get("state"))
		if err != nil {
			httpx.WriteError(response, request, http.StatusBadRequest, "invalid_oauth_callback", "Google login could not be completed.")
			return
		}
		http.Redirect(
			response,
			request,
			deps.Config.Runtime.WebURL+withQuery(returnTo, "authError", "access_denied"),
			http.StatusFound,
		)
		return
	}
	completion, err := deps.OAuth.Complete(
		request.Context(),
		request.URL.Query().Get("state"),
		request.URL.Query().Get("code"),
		deviceLabel(request),
		httpx.RequestID(request.Context()),
		quota.HashIPBytes(deps.Config.Cookies.SigningKey, httpx.ClientIP(request.Context())),
	)
	if errors.Is(err, users.ErrIdentityConflict) || errors.Is(err, users.ErrGuestConflict) {
		httpx.WriteError(response, request, http.StatusConflict, "identity_conflict", "Account identity could not be linked.")
		return
	}
	if errors.Is(err, identityoauth.ErrIdentityMismatch) {
		httpx.WriteError(response, request, http.StatusForbidden, "identity_conflict", "Google account does not match the current user.")
		return
	}
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_oauth_callback", "Google login could not be completed.")
		return
	}
	if _, err := deps.Browser.Issue(response, completion.Credentials); err != nil {
		deps.internalError(response, request, err)
		return
	}
	http.Redirect(response, request, deps.Config.Runtime.WebURL+completion.ReturnTo, http.StatusFound)
}

func (deps Dependencies) testAuthorize(response http.ResponseWriter, request *http.Request) {
	if !deps.Config.OAuth.TestMode {
		http.NotFound(response, request)
		return
	}
	state := request.URL.Query().Get("state")
	if state == "" {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "OAuth state is required.")
		return
	}
	switch request.URL.Query().Get("decision") {
	case "approve":
		http.Redirect(
			response,
			request,
			withQuery(deps.Config.OAuth.CallbackURL, "state", state, "code", "glazz-e2e-approved"),
			http.StatusFound,
		)
	case "deny":
		http.Redirect(
			response,
			request,
			withQuery(deps.Config.OAuth.CallbackURL, "state", state, "error", "access_denied"),
			http.StatusFound,
		)
	default:
		escapedState := url.QueryEscape(state)
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(response, `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Glazz test authorization</title></head>
<body><main><h1>Test authorization</h1><p>%s</p>
<a href="?state=%s&amp;decision=approve">Approve</a>
<a href="?state=%s&amp;decision=deny">Deny</a></main></body></html>`,
			html.EscapeString(deps.Config.OAuth.TestEmail), escapedState, escapedState)
	}
}

func withQuery(raw string, values ...string) string {
	target, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := target.Query()
	for index := 0; index+1 < len(values); index += 2 {
		query.Set(values[index], values[index+1])
	}
	target.RawQuery = query.Encode()
	return target.String()
}

func (deps Dependencies) startReauthentication(response http.ResponseWriter, request *http.Request) {
	if deps.OAuth == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Google login is not configured.")
		return
	}
	actor, _ := browser.CurrentActor(request.Context())
	authorizationURL, err := deps.OAuth.Start(request.Context(), identityoauth.StartInput{
		ReturnTo: "/settings/security", TermsAccepted: true, PrivacyAccepted: true,
		Locale: locale(request), ExpectedUserID: &actor.UserID,
	})
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Reauthentication could not be started.")
		return
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]string{"authorizationUrl": authorizationURL})
}

func (deps Dependencies) refresh(response http.ResponseWriter, request *http.Request) {
	raw, err := browser.RefreshToken(request)
	if err != nil {
		httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Refresh token is required.")
		return
	}
	credentials, err := deps.Sessions.Rotate(request.Context(), raw)
	if errors.Is(err, sessions.ErrRefreshReuse) {
		deps.Browser.Clear(response)
		httpx.WriteError(response, request, http.StatusConflict, "refresh_token_reused", "Session revoked.")
		return
	}
	if err != nil {
		deps.Browser.Clear(response)
		httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Refresh token is invalid.")
		return
	}
	if _, err := deps.Browser.Issue(response, credentials); err != nil {
		deps.internalError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (deps Dependencies) logout(response http.ResponseWriter, request *http.Request) {
	actor, _ := browser.CurrentActor(request.Context())
	if _, err := deps.Sessions.Revoke(request.Context(), actor.UserID, actor.SessionID); err != nil {
		deps.internalError(response, request, err)
		return
	}
	deps.Browser.Clear(response)
	response.WriteHeader(http.StatusNoContent)
}

func (deps Dependencies) me(response http.ResponseWriter, request *http.Request) {
	actor, _ := browser.CurrentActor(request.Context())
	user, err := deps.Database.Queries().FindUserByID(request.Context(), actor.UserID)
	if err != nil {
		httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "User is not active.")
		return
	}
	permissions := []string{"chat:use", "sessions:manage"}
	if user.Role == "admin" {
		permissions = append(permissions, "admin:access")
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]any{
		"id": user.ID, "email": user.Email, "displayName": user.DisplayName,
		"avatarUrl": user.AvatarUrl, "locale": user.Locale, "role": user.Role,
		"plan": user.Plan, "permissions": permissions,
	})
}

func (deps Dependencies) listSessions(response http.ResponseWriter, request *http.Request) {
	actor, _ := browser.CurrentActor(request.Context())
	records, err := deps.Sessions.List(request.Context(), actor.UserID)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id": record.ID, "current": record.ID == actor.SessionID, "deviceLabel": record.DeviceLabel,
			"createdAt": record.CreatedAt.Time, "lastSeenAt": record.LastSeenAt.Time,
			"expiresAt": record.RefreshExpiresAt.Time,
		})
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]any{"items": items})
}

func (deps Dependencies) revokeSession(response http.ResponseWriter, request *http.Request) {
	actor, _ := browser.CurrentActor(request.Context())
	sessionID, err := uuid.Parse(chi.URLParam(request, "sessionId"))
	if err != nil {
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Session was not found.")
		return
	}
	revoked, err := deps.Sessions.Revoke(request.Context(), actor.UserID, sessionID)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	if !revoked {
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Session was not found.")
		return
	}
	if sessionID == actor.SessionID {
		deps.Browser.Clear(response)
	}
	response.WriteHeader(http.StatusNoContent)
}

func (deps Dependencies) internalError(response http.ResponseWriter, request *http.Request, err error) {
	deps.Logger.ErrorContext(
		request.Context(), "request failed", "request_id", httpx.RequestID(request.Context()),
		"error_type", fmt.Sprintf("%T", err),
	)
	httpx.WriteError(response, request, http.StatusInternalServerError, "internal_error", "Request could not be processed.")
}

func locale(request *http.Request) string {
	if strings.HasPrefix(strings.ToLower(request.Header.Get("Accept-Language")), "es") {
		return "es"
	}
	return "en"
}

func deviceLabel(request *http.Request) string {
	value := strings.TrimSpace(request.UserAgent())
	if len(value) > 200 {
		return value[:200]
	}
	return value
}
