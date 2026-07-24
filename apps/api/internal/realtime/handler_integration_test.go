//go:build integration

package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
)

func TestWebSocketTicketLifecycle(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := redisx.Open(ctx, config.Redis{
		URL: runtimeConfig.Redis.URL, Prefix: "glazz-ws-integration-" + uuid.NewString(),
		HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	timeSource := clock.UTC{}
	idSource := ids.NewUUIDv7()
	tickets := NewTickets(store, timeSource, 30*time.Second)
	broker := NewBroker(store, idSource, timeSource)
	handler := NewHandler(tickets, broker, nil, timeSource, []string{"http://localhost:3000"})
	actor := conversations.Actor{Type: conversations.ActorGuest, ID: uuid.New()}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		handler.Serve(response, request, actor)
	}))
	defer server.Close()

	ticket, err := tickets.Issue(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(strings.Replace(server.URL, "http://", "ws://", 1))
	query := endpoint.Query()
	query.Set("ticket", ticket.Value)
	endpoint.RawQuery = query.Encode()
	connection, _, err := websocket.Dial(ctx, endpoint.String(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://localhost:3000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ready Event
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := wsjson.Read(readCtx, connection, &ready); err != nil {
		t.Fatal(err)
	}
	if ready.Type != "connection.ready" || ready.Sequence != 1 {
		t.Fatalf("ready event = %#v", ready)
	}
	_ = connection.Close(websocket.StatusNormalClosure, "test complete")

	replayed, response, err := websocket.Dial(ctx, endpoint.String(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://localhost:3000"}},
	})
	if replayed != nil {
		replayed.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replayed ticket = response %#v, error %v", response, err)
	}
}
