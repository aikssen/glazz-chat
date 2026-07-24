package server

import (
	"net/http"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
)

func (deps Dependencies) createWebSocketTicket(
	response http.ResponseWriter,
	request *http.Request,
) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	if deps.Tickets == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Realtime service is unavailable.")
		return
	}
	ticket, err := deps.Tickets.Issue(request.Context(), actor)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(response, http.StatusCreated, ticket)
}

func (deps Dependencies) websocket(response http.ResponseWriter, request *http.Request) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	if deps.Realtime == nil {
		httpx.WriteError(response, request, http.StatusServiceUnavailable, "service_unavailable", "Realtime service is unavailable.")
		return
	}
	deps.Realtime.Serve(response, request, actor)
}
