package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/admin"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
	"github.com/aikssen/glazz-chat/apps/api/internal/privacy"
)

func (deps Dependencies) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if deps.Admin == nil {
			httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Administration is unavailable.")
			return
		}
		actor, ok := browser.CurrentActor(request.Context())
		if !ok {
			httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Session is not active.")
			return
		}
		user, err := deps.Database.Queries().FindUserByID(request.Context(), actor.UserID)
		if err != nil || user.Status != "active" || user.Role != "admin" {
			httpx.WriteError(response, request, http.StatusForbidden, "forbidden", "Administrator access is required.")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (deps Dependencies) requireRecent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		actor, ok := browser.CurrentActor(request.Context())
		if !ok {
			httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Session is not active.")
			return
		}
		session, err := deps.Database.Queries().GetAuthSession(request.Context(), actor.SessionID)
		if err != nil || !session.RecentAuthAt.Valid ||
			time.Since(session.RecentAuthAt.Time) > deps.Config.Auth.RecentAuthTTL {
			httpx.WriteError(response, request, http.StatusPreconditionRequired, "recent_auth_required", "Recent authentication is required.")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (deps Dependencies) adminListSettings(response http.ResponseWriter, request *http.Request) {
	items, err := deps.Admin.ListSettings(request.Context())
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]any{"items": items})
}

func (deps Dependencies) adminUpdateSetting(response http.ResponseWriter, request *http.Request) {
	version, ok := matchVersion(response, request)
	if !ok {
		return
	}
	var payload struct {
		Value json.RawMessage `json:"value"`
	}
	if decodeJSON(request, &payload) != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Setting update is invalid.")
		return
	}
	actor, _ := browser.CurrentActor(request.Context())
	setting, err := deps.Admin.UpdateSetting(
		request.Context(), actor.UserID, chi.URLParam(request, "key"), payload.Value,
		version, httpx.RequestID(request.Context()),
	)
	if err != nil {
		deps.adminError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, setting)
}

func (deps Dependencies) adminListModels(response http.ResponseWriter, request *http.Request) {
	items, err := deps.Admin.ListModels(request.Context())
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, map[string]any{"items": items})
}

func (deps Dependencies) adminUpdateModel(response http.ResponseWriter, request *http.Request) {
	version, ok := matchVersion(response, request)
	if !ok {
		return
	}
	modelID, err := uuid.Parse(chi.URLParam(request, "modelId"))
	if err != nil {
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Model was not found.")
		return
	}
	var payload struct {
		Enabled    *bool     `json:"enabled"`
		Audience   *[]string `json:"audience"`
		DefaultFor *[]string `json:"defaultFor"`
		Order      *int32    `json:"order"`
	}
	if decodeJSON(request, &payload) != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Model update is invalid.")
		return
	}
	actor, _ := browser.CurrentActor(request.Context())
	model, err := deps.Admin.UpdateModel(
		request.Context(), actor.UserID, modelID,
		admin.ModelUpdate{
			Enabled: payload.Enabled, Audience: payload.Audience,
			DefaultFor: payload.DefaultFor, Order: payload.Order,
		},
		version, httpx.RequestID(request.Context()),
	)
	if err != nil {
		deps.adminError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, model)
}

func (deps Dependencies) adminSyncModels(response http.ResponseWriter, request *http.Request) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if len(key) < 16 || len(key) > 128 {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Idempotency-Key is invalid.")
		return
	}
	id, err := deps.IDs.New()
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	payload, _ := json.Marshal(map[string]string{"providerCode": "fake"})
	if err := deps.Database.Queries().EnqueueOutboxEvent(request.Context(), store.EnqueueOutboxEventParams{
		ID: id, EventType: "models.sync", Payload: payload, IdempotencyKey: key,
		AvailableAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}); err != nil {
		deps.internalError(response, request, err)
		return
	}
	event, err := deps.Database.Queries().GetOutboxEventByIdempotencyKey(request.Context(), key)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	status := "pending"
	if event.ProcessedAt.Valid {
		status = "completed"
	} else if event.DeadLetteredAt.Valid {
		status = "failed"
	}
	httpx.WriteJSON(response, http.StatusAccepted, map[string]any{"id": event.ID, "status": status})
}

func (deps Dependencies) adminListUsers(response http.ResponseWriter, request *http.Request) {
	limit, err := queryLimit(request, 30)
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Pagination is invalid.")
		return
	}
	page, err := deps.Admin.ListUsers(
		request.Context(), request.URL.Query().Get("query"),
		request.URL.Query().Get("after"), limit,
	)
	if err != nil {
		deps.adminError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, page)
}

func (deps Dependencies) adminUpdateUserRole(response http.ResponseWriter, request *http.Request) {
	version, ok := matchVersion(response, request)
	if !ok {
		return
	}
	userID, err := uuid.Parse(chi.URLParam(request, "userId"))
	if err != nil {
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "User was not found.")
		return
	}
	var payload struct {
		Role string `json:"role"`
	}
	if decodeJSON(request, &payload) != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Role update is invalid.")
		return
	}
	actor, _ := browser.CurrentActor(request.Context())
	user, err := deps.Admin.UpdateUserRole(
		request.Context(), actor.UserID, userID, payload.Role, version,
		httpx.RequestID(request.Context()),
	)
	if err != nil {
		deps.adminError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, user)
}

func (deps Dependencies) adminUsage(response http.ResponseWriter, request *http.Request) {
	end := time.Now().UTC()
	start := end.Add(-30 * 24 * time.Hour)
	var err error
	if raw := request.URL.Query().Get("from"); raw != "" {
		start, err = time.Parse(time.RFC3339, raw)
	}
	if raw := request.URL.Query().Get("to"); err == nil && raw != "" {
		end, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Usage period is invalid.")
		return
	}
	usage, err := deps.Admin.Usage(request.Context(), start, end)
	if err != nil {
		deps.adminError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, usage)
}

func (deps Dependencies) adminAudit(response http.ResponseWriter, request *http.Request) {
	limit, err := queryLimit(request, 30)
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Pagination is invalid.")
		return
	}
	page, err := deps.Admin.Audit(request.Context(), request.URL.Query().Get("after"), limit)
	if err != nil {
		deps.adminError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, page)
}

func (deps Dependencies) requestAccountDeletion(response http.ResponseWriter, request *http.Request) {
	if deps.Privacy == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Account deletion is unavailable.")
		return
	}
	var payload struct {
		Confirmation string `json:"confirmation"`
	}
	if decodeJSON(request, &payload) != nil || payload.Confirmation != "DELETE" {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Deletion confirmation is invalid.")
		return
	}
	actor, _ := browser.CurrentActor(request.Context())
	job, err := deps.Privacy.Request(request.Context(), actor.UserID)
	if err != nil {
		deps.privacyError(response, request, err)
		return
	}
	deps.Browser.Clear(response)
	httpx.WriteJSON(response, http.StatusAccepted, job)
}

func (deps Dependencies) getAccountDeletion(response http.ResponseWriter, request *http.Request) {
	if deps.Privacy == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Account deletion is unavailable.")
		return
	}
	actor, _ := browser.CurrentActor(request.Context())
	job, err := deps.Privacy.Get(request.Context(), actor.UserID)
	if err != nil {
		deps.privacyError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, job)
}

func (deps Dependencies) adminError(response http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, admin.ErrInvalid):
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Administrative input is invalid.")
	case errors.Is(err, admin.ErrNotFound):
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Administrative resource was not found.")
	case errors.Is(err, admin.ErrConflict):
		httpx.WriteError(response, request, http.StatusConflict, "conflict", "Resource changed or the operation would violate a required invariant.")
	default:
		deps.internalError(response, request, err)
	}
}

func (deps Dependencies) privacyError(response http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, privacy.ErrNotFound):
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Deletion request was not found.")
	case errors.Is(err, privacy.ErrConflict):
		httpx.WriteError(response, request, http.StatusConflict, "conflict", "Account deletion conflicts with current state.")
	default:
		deps.internalError(response, request, err)
	}
}

func matchVersion(response http.ResponseWriter, request *http.Request) (int32, bool) {
	raw := request.Header.Get("If-Match")
	if len(raw) < 3 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		httpx.WriteError(response, request, http.StatusPreconditionRequired, "precondition_required", "If-Match is required.")
		return 0, false
	}
	value, err := strconv.ParseInt(raw[1:len(raw)-1], 10, 32)
	if err != nil || value <= 0 {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "If-Match is invalid.")
		return 0, false
	}
	return int32(value), true
}
