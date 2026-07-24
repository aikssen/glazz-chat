//go:build integration

package realtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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
	secondaryBroker := NewBroker(store, idSource, timeSource)
	if _, err := secondaryBroker.Emit(
		ctx, actor, "conversation.updated", "req_cross_instance",
		map[string]any{"conversationId": uuid.NewString()},
	); err != nil {
		t.Fatal(err)
	}
	var crossInstance Event
	if err := wsjson.Read(readCtx, connection, &crossInstance); err != nil {
		t.Fatal(err)
	}
	if crossInstance.Type != "conversation.updated" {
		t.Fatalf("cross-instance event = %#v", crossInstance)
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

	for index := 0; index < replayLimit+10; index++ {
		if _, err := broker.Emit(
			ctx, actor, "usage.updated", "req_fill_replay",
			map[string]int{"index": index},
		); err != nil {
			t.Fatal(err)
		}
	}
	resumeTicket, err := tickets.Issue(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	query.Set("ticket", resumeTicket.Value)
	endpoint.RawQuery = query.Encode()
	resumed, _, err := websocket.Dial(ctx, endpoint.String(), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://localhost:3000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.CloseNow()
	if err := wsjson.Read(readCtx, resumed, &ready); err != nil {
		t.Fatal(err)
	}
	resumePayload := RawEvent{
		Version: ProtocolVersion, Type: "connection.resume",
		EventID: "evt_resume_integration", RequestID: "req_resume_integration",
		IdempotencyKey: "idem-resume-integration", OccurredAt: timeSource.Now(),
		Payload: []byte(`{"lastSequence":1}`),
	}
	if err := wsjson.Write(ctx, resumed, resumePayload); err != nil {
		t.Fatal(err)
	}
	var resync Event
	if err := wsjson.Read(readCtx, resumed, &resync); err != nil {
		t.Fatal(err)
	}
	if resync.Type != "connection.resync_required" {
		t.Fatalf("resync event = %#v", resync)
	}
}

func TestWebSocketRejectsDisallowedOrigin(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := redisx.Open(ctx, config.Redis{
		URL: runtimeConfig.Redis.URL, Prefix: "glazz-ws-origin-" + uuid.NewString(),
		HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	timeSource := clock.UTC{}
	tickets := NewTickets(store, timeSource, 30*time.Second)
	actor := conversations.Actor{Type: conversations.ActorGuest, ID: uuid.New()}
	handler := NewHandler(
		tickets, NewBroker(store, ids.NewUUIDv7(), timeSource), nil, timeSource,
		[]string{"http://localhost:3000"},
	)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		handler.Serve(response, request, actor)
	}))
	defer server.Close()
	ticket, err := tickets.Issue(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := strings.Replace(server.URL, "http://", "ws://", 1) + "?ticket=" +
		url.QueryEscape(ticket.Value)
	connection, response, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"https://evil.example"}},
	})
	if connection != nil {
		connection.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("origin denial = response %#v, error %v", response, err)
	}
}

func TestWebSocketHeartbeatTimeout(t *testing.T) {
	fixture := newWebSocketFixture(t)
	fixture.handler.heartbeatEvery = 10 * time.Millisecond
	fixture.handler.heartbeatWait = 25 * time.Millisecond

	connection := fixture.dial(t)
	defer connection.CloseNow()
	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		var event Event
		err := wsjson.Read(readCtx, connection, &event)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("connection did not close after heartbeat timeout")
			}
			return
		}
	}
}

func TestWebSocketReconnectLoad(t *testing.T) {
	fixture := newWebSocketFixture(t)
	const connections = 24
	var wait sync.WaitGroup
	errorsFound := make(chan error, connections)
	wait.Add(connections)
	for index := 0; index < connections; index++ {
		go func() {
			defer wait.Done()
			connection, err := fixture.connect()
			if err != nil {
				errorsFound <- err
				return
			}
			defer connection.CloseNow()
			readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var ready Event
			if err := wsjson.Read(readCtx, connection, &ready); err != nil {
				errorsFound <- err
				return
			}
			if ready.Type != "connection.ready" {
				errorsFound <- errors.New("first event was not connection.ready")
			}
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
}

type webSocketFixture struct {
	t       *testing.T
	ctx     context.Context
	store   *redisx.Client
	tickets *Tickets
	handler *Handler
	server  *httptest.Server
	actor   conversations.Actor
}

func newWebSocketFixture(t *testing.T) *webSocketFixture {
	t.Helper()
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := redisx.Open(ctx, config.Redis{
		URL: runtimeConfig.Redis.URL, Prefix: "glazz-ws-lifecycle-" + uuid.NewString(),
		HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	timeSource := clock.UTC{}
	tickets := NewTickets(store, timeSource, 30*time.Second)
	actor := conversations.Actor{Type: conversations.ActorUser, ID: uuid.New()}
	handler := NewHandler(
		tickets, NewBroker(store, ids.NewUUIDv7(), timeSource), nil, timeSource,
		[]string{"http://localhost:3000"},
	)
	fixture := &webSocketFixture{
		t: t, ctx: ctx, store: store, tickets: tickets, handler: handler, actor: actor,
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			handler.Serve(response, request, actor)
		},
	))
	t.Cleanup(func() {
		fixture.server.Close()
		_ = store.Close()
	})
	return fixture
}

func (fixture *webSocketFixture) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	connection, err := fixture.connect()
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func (fixture *webSocketFixture) connect() (*websocket.Conn, error) {
	ticket, err := fixture.tickets.Issue(fixture.ctx, fixture.actor)
	if err != nil {
		return nil, err
	}
	endpoint := strings.Replace(fixture.server.URL, "http://", "ws://", 1) +
		"?ticket=" + url.QueryEscape(ticket.Value)
	connection, _, err := websocket.Dial(fixture.ctx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://localhost:3000"}},
	})
	return connection, err
}
