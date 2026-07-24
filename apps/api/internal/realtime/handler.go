package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/redis/go-redis/v9"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
)

const (
	maxCommandBytes   = 64 << 10
	writeQueueSize    = 64
	heartbeatInterval = 20 * time.Second
	heartbeatTimeout  = 50 * time.Second
	writeTimeout      = 10 * time.Second
)

type CommandProcessor interface {
	Prepare(context.Context, conversations.Actor, RawEvent) (func(), error)
}

type Handler struct {
	tickets        *Tickets
	broker         *Broker
	processor      CommandProcessor
	clock          clock.Clock
	originPatterns []string
}

func NewHandler(
	tickets *Tickets,
	broker *Broker,
	processor CommandProcessor,
	timeSource clock.Clock,
	allowedOrigins []string,
) *Handler {
	return &Handler{
		tickets: tickets, broker: broker, processor: processor, clock: timeSource,
		originPatterns: originPatterns(allowedOrigins),
	}
}

func (handler *Handler) Serve(
	response http.ResponseWriter,
	request *http.Request,
	actor conversations.Actor,
) {
	ticket := request.URL.Query().Get("ticket")
	if err := handler.tickets.Consume(request.Context(), ticket, actor); err != nil {
		http.Error(response, "WebSocket ticket is invalid.", http.StatusUnauthorized)
		return
	}
	connection, err := websocket.Accept(response, request, &websocket.AcceptOptions{
		OriginPatterns: handler.originPatterns, CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	connection.SetReadLimit(maxCommandBytes)
	defer connection.CloseNow()

	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	subscription, err := handler.broker.Subscribe(ctx, actor)
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "realtime dependency unavailable")
		return
	}
	defer subscription.Close()

	outgoing := make(chan string, writeQueueSize)
	failures := make(chan error, 3)
	lastPong := atomic.Int64{}
	lastPong.Store(handler.clock.Now().UnixNano())

	go handler.writeLoop(ctx, connection, outgoing, failures)
	go handler.subscriptionLoop(ctx, subscription.Channel(), outgoing, failures)
	go handler.heartbeatLoop(ctx, actor, lastPong.Load, failures)

	if _, err := handler.broker.Emit(ctx, actor, "connection.ready", requestID(request), map[string]any{
		"actorType": string(actor.Type), "heartbeatIntervalMs": heartbeatInterval.Milliseconds(),
		"replayWindow": replayLimit, "serverTime": handler.clock.Now().UTC(),
	}); err != nil {
		_ = connection.Close(websocket.StatusInternalError, "realtime dependency unavailable")
		return
	}

	readError := make(chan error, 1)
	go func() {
		readError <- handler.readLoop(ctx, connection, actor, outgoing, &lastPong)
	}()
	select {
	case err := <-readError:
		handler.closeForError(connection, err)
	case err := <-failures:
		handler.closeForError(connection, err)
	case <-ctx.Done():
		_ = connection.Close(websocket.StatusGoingAway, "server shutting down")
	}
}

func (handler *Handler) readLoop(
	ctx context.Context,
	connection *websocket.Conn,
	actor conversations.Actor,
	outgoing chan<- string,
	lastPong *atomic.Int64,
) error {
	for {
		var event RawEvent
		if err := wsjson.Read(ctx, connection, &event); err != nil {
			return err
		}
		if err := validateClientEvent(event); err != nil {
			if _, emitErr := handler.broker.Emit(
				ctx, actor, "command.rejected", event.RequestID,
				map[string]any{
					"commandEventId": event.EventID, "code": "invalid_command",
					"message": "Command envelope is invalid.",
				},
			); emitErr != nil {
				return emitErr
			}
			continue
		}
		switch event.Type {
		case "heartbeat.pong":
			lastPong.Store(handler.clock.Now().UnixNano())
		case "connection.resume":
			var payload struct {
				LastSequence int64 `json:"lastSequence"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.LastSequence < 0 {
				return errors.New("invalid resume payload")
			}
			replay, err := handler.broker.Replay(ctx, actor, payload.LastSequence)
			if err != nil {
				return err
			}
			for _, encoded := range replay {
				if !enqueue(outgoing, encoded) {
					return errors.New("realtime write queue is full")
				}
			}
		case "chat.generate", "chat.cancel":
			if handler.processor == nil {
				return errors.New("chat processor is unavailable")
			}
			start, err := handler.processor.Prepare(ctx, actor, event)
			if err != nil {
				if _, emitErr := handler.broker.Emit(
					ctx, actor, "command.rejected", event.RequestID,
					map[string]any{
						"commandEventId": event.EventID, "code": realtimeCode(err),
						"message": "Command could not be processed.",
					},
				); emitErr != nil {
					return emitErr
				}
				continue
			}
			if _, err := handler.broker.Emit(
				ctx, actor, "command.acknowledged", event.RequestID,
				map[string]string{"commandEventId": event.EventID},
			); err != nil {
				return err
			}
			if start != nil {
				start()
			}
		}
	}
}

func (handler *Handler) writeLoop(
	ctx context.Context,
	connection *websocket.Conn,
	outgoing <-chan string,
	failures chan<- error,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case encoded := <-outgoing:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := connection.Write(writeCtx, websocket.MessageText, []byte(encoded))
			cancel()
			if err != nil {
				select {
				case failures <- err:
				default:
				}
				return
			}
		}
	}
}

func (handler *Handler) subscriptionLoop(
	ctx context.Context,
	messages <-chan *redis.Message,
	outgoing chan<- string,
	failures chan<- error,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			if !enqueue(outgoing, message.Payload) {
				select {
				case failures <- errors.New("realtime write queue is full"):
				default:
				}
				return
			}
		}
	}
}

func (handler *Handler) heartbeatLoop(
	ctx context.Context,
	actor conversations.Actor,
	lastPong func() int64,
	failures chan<- error,
) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if handler.clock.Now().Sub(time.Unix(0, lastPong())) > heartbeatTimeout {
				select {
				case failures <- errors.New("heartbeat timed out"):
				default:
				}
				return
			}
			if _, err := handler.broker.Emit(
				ctx, actor, "heartbeat.ping", "", map[string]string{
					"heartbeatId": fmt.Sprintf("%d", handler.clock.Now().UnixNano()),
				},
			); err != nil {
				select {
				case failures <- err:
				default:
				}
				return
			}
		}
	}
}

func (handler *Handler) closeForError(connection *websocket.Conn, err error) {
	status := websocket.CloseStatus(err)
	if status != -1 {
		return
	}
	_ = connection.Close(websocket.StatusPolicyViolation, "connection closed")
}

func validateClientEvent(event RawEvent) error {
	if event.Version != ProtocolVersion ||
		len(event.EventID) < 8 || len(event.EventID) > 128 ||
		len(event.RequestID) < 8 || len(event.RequestID) > 128 ||
		len(event.IdempotencyKey) < 16 || len(event.IdempotencyKey) > 128 ||
		event.OccurredAt.IsZero() {
		return errors.New("invalid realtime envelope")
	}
	switch event.Type {
	case "connection.resume", "chat.generate", "chat.cancel", "heartbeat.pong":
		return nil
	default:
		return errors.New("unsupported realtime command")
	}
}

func enqueue(queue chan<- string, value string) bool {
	select {
	case queue <- value:
		return true
	default:
		return false
	}
}

func originPatterns(origins []string) []string {
	result := make([]string, 0, len(origins))
	for _, origin := range origins {
		parsed, err := url.Parse(origin)
		if err == nil && parsed.Host != "" {
			result = append(result, parsed.Host)
		}
	}
	return result
}

func requestID(request *http.Request) string {
	value := request.Header.Get("X-Request-ID")
	if value == "" {
		return "req_websocket"
	}
	return value
}

func realtimeCode(err error) string {
	value := strings.ToLower(err.Error())
	switch {
	case strings.Contains(value, "quota"):
		return "quota_exceeded"
	case strings.Contains(value, "concurrent"):
		return "concurrency_exceeded"
	case strings.Contains(value, "not found"):
		return "not_found"
	case strings.Contains(value, "forbidden"):
		return "forbidden"
	default:
		return "conflict"
	}
}
