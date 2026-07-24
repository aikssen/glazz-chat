package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
)

func (deps Dependencies) actor(request *http.Request) (conversations.Actor, error) {
	if deps.ResolveUser != nil {
		user, err := deps.ResolveUser(request)
		if err == nil {
			return conversations.Actor{Type: conversations.ActorUser, ID: user.UserID}, nil
		}
		if !errors.Is(err, browser.ErrUnauthenticated) {
			return conversations.Actor{}, err
		}
	}
	guestID, err := deps.Guests.ID(request.Context(), request)
	if err != nil || guestID == nil {
		return conversations.Actor{}, browser.ErrUnauthenticated
	}
	return conversations.Actor{Type: conversations.ActorGuest, ID: *guestID}, nil
}

func (deps Dependencies) withActorCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		actor, err := deps.actor(request)
		if err != nil {
			deps.actorError(response, request, err)
			return
		}
		if actor.Type == conversations.ActorUser {
			deps.Browser.CSRF(next).ServeHTTP(response, request)
			return
		}
		if _, err := request.Cookie(browser.AccessCookie); err == nil {
			deps.Browser.Clear(response)
		}
		deps.Guests.CSRF(next).ServeHTTP(response, request)
	}
}

func (deps Dependencies) listModels(response http.ResponseWriter, request *http.Request) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	items, err := deps.Models.List(request.Context(), string(actor.Type))
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	defaultModel, err := deps.Models.Default(request.Context(), string(actor.Type))
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "private, max-age=60")
	httpx.WriteJSON(response, http.StatusOK, map[string]any{
		"items": items, "defaultModelId": defaultModel.Model.ID,
	})
}

func (deps Dependencies) createConversation(response http.ResponseWriter, request *http.Request) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	var payload struct {
		Title   string     `json:"title"`
		ModelID *uuid.UUID `json:"modelId"`
	}
	if request.ContentLength != 0 {
		if err := decodeJSON(request, &payload); err != nil {
			httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Conversation input is invalid.")
			return
		}
	}
	conversation, err := deps.Chats.Create(request.Context(), actor, conversations.CreateInput{
		Title: payload.Title, ModelID: payload.ModelID,
	})
	if err != nil {
		deps.conversationError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusCreated, conversation)
}

func (deps Dependencies) listConversations(response http.ResponseWriter, request *http.Request) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	limit, err := queryLimit(request, 20)
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Pagination is invalid.")
		return
	}
	includeArchived, err := queryBool(request, "archived")
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Archived filter is invalid.")
		return
	}
	page, err := deps.Chats.List(request.Context(), actor, conversations.ListInput{
		Limit: limit, Cursor: request.URL.Query().Get("after"),
		Search: request.URL.Query().Get("query"), IncludeArchived: includeArchived,
	})
	if err != nil {
		deps.conversationError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(response, http.StatusOK, page)
}

func (deps Dependencies) getConversation(response http.ResponseWriter, request *http.Request) {
	actor, conversationID, ok := deps.conversationRequest(response, request)
	if !ok {
		return
	}
	conversation, err := deps.Chats.Get(request.Context(), actor, conversationID)
	if err != nil {
		deps.conversationError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "private, no-cache")
	response.Header().Set("ETag", fmt.Sprintf(`W/"%d"`, conversation.UpdatedAt.UnixNano()))
	httpx.WriteJSON(response, http.StatusOK, conversation)
}

func (deps Dependencies) updateConversation(response http.ResponseWriter, request *http.Request) {
	actor, conversationID, ok := deps.conversationRequest(response, request)
	if !ok {
		return
	}
	var payload struct {
		Title    *string    `json:"title"`
		Archived *bool      `json:"archived"`
		ModelID  *uuid.UUID `json:"modelId"`
	}
	if err := decodeJSON(request, &payload); err != nil ||
		(payload.Title == nil && payload.Archived == nil && payload.ModelID == nil) {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Conversation update is invalid.")
		return
	}
	conversation, err := deps.Chats.Update(request.Context(), actor, conversationID, conversations.UpdateInput{
		Title: payload.Title, Archived: payload.Archived, ModelID: payload.ModelID,
	})
	if err != nil {
		deps.conversationError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, conversation)
}

func (deps Dependencies) deleteConversation(response http.ResponseWriter, request *http.Request) {
	actor, conversationID, ok := deps.conversationRequest(response, request)
	if !ok {
		return
	}
	if err := deps.Chats.Delete(request.Context(), actor, conversationID); err != nil {
		deps.conversationError(response, request, err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (deps Dependencies) listConversationMessages(response http.ResponseWriter, request *http.Request) {
	actor, conversationID, ok := deps.conversationRequest(response, request)
	if !ok {
		return
	}
	limit, err := queryLimit(request, 50)
	if err != nil {
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Pagination is invalid.")
		return
	}
	var before *int32
	if value := request.URL.Query().Get("after"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed <= 0 {
			httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Pagination is invalid.")
			return
		}
		converted := int32(parsed)
		before = &converted
	}
	page, err := deps.Chats.Messages(request.Context(), actor, conversationID, limit, before)
	if err != nil {
		deps.conversationError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "private, no-cache")
	httpx.WriteJSON(response, http.StatusOK, page)
}

func (deps Dependencies) conversationRequest(
	response http.ResponseWriter,
	request *http.Request,
) (conversations.Actor, uuid.UUID, bool) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return conversations.Actor{}, uuid.Nil, false
	}
	conversationID, err := uuid.Parse(chi.URLParam(request, "conversationId"))
	if err != nil {
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Conversation was not found.")
		return conversations.Actor{}, uuid.Nil, false
	}
	return actor, conversationID, true
}

func (deps Dependencies) actorError(response http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, browser.ErrUnauthenticated) {
		httpx.WriteError(response, request, http.StatusUnauthorized, "unauthenticated", "Authentication is required.")
		return
	}
	deps.internalError(response, request, err)
}

func (deps Dependencies) conversationError(response http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, conversations.ErrInvalid):
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Conversation input is invalid.")
	case errors.Is(err, conversations.ErrNotFound):
		httpx.WriteError(response, request, http.StatusNotFound, "not_found", "Conversation was not found.")
	case errors.Is(err, conversations.ErrConflict):
		httpx.WriteError(response, request, http.StatusConflict, "conflict", "Conversation conflicts with its current state.")
	case errors.Is(err, conversations.ErrGuestScope):
		httpx.WriteError(response, request, http.StatusForbidden, "forbidden", "Operation is unavailable to guests.")
	case errors.Is(err, models.ErrNotSelectable):
		httpx.WriteError(response, request, http.StatusBadRequest, "invalid_request", "Model is unavailable.")
	default:
		deps.internalError(response, request, err)
	}
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func queryLimit(request *http.Request, fallback int32) (int32, error) {
	value := request.URL.Query().Get("limit")
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 1 || parsed > 100 {
		return 0, errors.New("invalid limit")
	}
	return int32(parsed), nil
}

func queryBool(request *http.Request, name string) (bool, error) {
	value := strings.TrimSpace(request.URL.Query().Get(name))
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}
