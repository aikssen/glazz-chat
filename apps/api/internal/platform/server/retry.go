package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/chat"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/quota"
)

func (deps Dependencies) retryGeneration(response http.ResponseWriter, request *http.Request) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	conversationID, err := uuid.Parse(chi.URLParam(request, "conversationId"))
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Conversation identifier is invalid.")
		return
	}
	if deps.ChatEngine == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Chat service is unavailable.")
		return
	}
	generation, start, err := deps.ChatEngine.Retry(
		request.Context(), actor, conversationID, request.Header.Get("Idempotency-Key"),
		httpx.RequestID(request.Context()),
	)
	switch {
	case errors.Is(err, chat.ErrInvalid):
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Retry request is invalid.")
		return
	case errors.Is(err, chat.ErrNotFound):
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Conversation was not found.")
		return
	case errors.Is(err, chat.ErrConflict):
		httpx.WriteError(response, request, http.StatusConflict, "conflict", "Latest generation cannot be retried.")
		return
	case errors.Is(err, quota.ErrExceeded), errors.Is(err, quota.ErrBusy):
		httpx.WriteError(response, request, http.StatusTooManyRequests, "quota_exceeded", "Chat quota is exhausted.")
		return
	case err != nil:
		deps.internalError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(response, http.StatusAccepted, generation)
	if start != nil {
		start()
	}
}
