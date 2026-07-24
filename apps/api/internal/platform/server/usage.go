package server

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/httpx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

func (deps Dependencies) usage(response http.ResponseWriter, request *http.Request) {
	actor, err := deps.actor(request)
	if err != nil {
		deps.actorError(response, request, err)
		return
	}
	now := time.Now().UTC()
	from := now.Truncate(24 * time.Hour)
	resetAt := from.Add(24 * time.Hour)
	messageLimit := int64(50)
	outputLimit := int64(50000)
	if deps.Settings != nil {
		snapshot, err := deps.Settings.Load(request.Context())
		if err != nil {
			deps.internalError(response, request, err)
			return
		}
		messageLimit = snapshot.UserMessageLimit
		outputLimit = snapshot.UserOutputTokenLimit
	}
	if actor.Type == conversations.ActorGuest {
		allowance, err := deps.Guests.Current(request.Context(), request)
		if err != nil {
			deps.actorError(response, request, err)
			return
		}
		active, err := deps.Database.Queries().CountActorActiveGenerations(
			request.Context(), store.CountActorActiveGenerationsParams{
				ActorType: string(actor.Type), ActorID: &actor.ID,
			},
		)
		if err != nil {
			deps.internalError(response, request, err)
			return
		}
		httpx.WriteJSON(response, http.StatusOK, usageResponse(
			int64(allowance.MessagesUsed), int64(allowance.MessageLimit),
			int64(allowance.OutputTokensUsed), int64(allowance.OutputTokenLimit),
			active, allowance.ExpiresAt,
		))
		return
	}
	current, err := deps.Database.Queries().GetActorUsage(
		request.Context(), store.GetActorUsageParams{
			ActorType: string(actor.Type), ActorID: actor.ID,
			FromAt: pgtype.Timestamptz{Time: from, Valid: true},
		},
	)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	active, err := deps.Database.Queries().CountActorActiveGenerations(
		request.Context(), store.CountActorActiveGenerationsParams{
			ActorType: string(actor.Type), ActorID: &actor.ID,
		},
	)
	if err != nil {
		deps.internalError(response, request, err)
		return
	}
	httpx.WriteJSON(response, http.StatusOK, usageResponse(
		current.Generations, messageLimit, current.OutputTokens, outputLimit, active, resetAt,
	))
}

func usageResponse(messagesUsed, messageLimit, tokensUsed, tokenLimit, active int64, reset time.Time) map[string]any {
	return map[string]any{
		"messages":     map[string]any{"used": messagesUsed, "limit": messageLimit, "resetAt": reset},
		"outputTokens": map[string]any{"used": tokensUsed, "limit": tokenLimit, "resetAt": reset},
		"concurrency":  map[string]int64{"used": active, "limit": 1},
	}
}
