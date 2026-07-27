package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	clientIPKey  contextKey = "client_ip"
)

type ErrorEnvelope struct {
	Error Problem `json:"error"`
}

type Problem struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

func WriteError(response http.ResponseWriter, request *http.Request, status int, code, message string) {
	response.Header().Set("Content-Type", "application/problem+json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(ErrorEnvelope{Error: Problem{
		Code:      code,
		Message:   message,
		RequestID: RequestID(request.Context()),
	}})
}

func WriteJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func ClientIP(ctx context.Context) netip.Addr {
	value, _ := ctx.Value(clientIPKey).(netip.Addr)
	return value
}

func RequestIDs(source ids.Source) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			requestID := request.Header.Get("X-Request-ID")
			if !validRequestID(requestID) {
				generated, err := source.New()
				if err != nil {
					WriteError(response, request, http.StatusInternalServerError, "internal_error", "Request could not be processed.")
					return
				}
				requestID = "req_" + generated.String()
			}
			response.Header().Set("X-Request-ID", requestID)
			ctx := context.WithValue(request.Context(), requestIDKey, requestID)
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(
						request.Context(),
						"request panic",
						"request_id", RequestID(request.Context()),
						"panic_type", fmt.Sprintf("%T", recovered),
						"stack", string(debug.Stack()),
					)
					WriteError(response, request, http.StatusInternalServerError, "internal_error", "Request could not be processed.")
				}
			}()
			next.ServeHTTP(response, request)
		})
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(response, request)
	})
}

func MaxBody(bytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			request.Body = http.MaxBytesReader(response, request.Body, bytes)
			next.ServeHTTP(response, request)
		})
	}
}

func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(response, request)
				return
			}
			ctx, cancel := context.WithTimeout(request.Context(), duration)
			defer cancel()
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin != "" {
				if _, ok := allowed[origin]; !ok {
					WriteError(response, request, http.StatusForbidden, "forbidden", "Origin is not allowed.")
					return
				}
				response.Header().Set("Access-Control-Allow-Origin", origin)
				response.Header().Set("Access-Control-Allow-Credentials", "true")
				response.Header().Add("Vary", "Origin")
			}
			if request.Method == http.MethodOptions {
				response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				response.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Request-ID, Idempotency-Key, If-Match")
				response.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func ClientIPs(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			remote := remoteAddress(request.RemoteAddr)
			client := remote
			if isTrusted(remote, trusted) {
				if forwarded, ok := firstForwardedAddress(request.Header.Get("X-Forwarded-For")); ok {
					client = forwarded
				}
			}
			ctx := context.WithValue(request.Context(), clientIPKey, client)
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

func MethodNotAllowed(response http.ResponseWriter, request *http.Request) {
	WriteError(response, request, http.StatusMethodNotAllowed, "invalid_request", "Method is not allowed.")
}

func NotFound(response http.ResponseWriter, request *http.Request) {
	WriteError(response, request, http.StatusNotFound, "not_found", "Resource was not found.")
}

func RoutePattern(request *http.Request) string {
	pattern := chi.RouteContext(request.Context()).RoutePattern()
	if pattern == "" {
		return "unmatched"
	}
	return pattern
}

func validRequestID(value string) bool {
	if len(value) < 4 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character == '-' || character == '_' || character == '.' ||
			character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func remoteAddress(value string) netip.Addr {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		host = value
	}
	address, _ := netip.ParseAddr(host)
	return address.Unmap()
}

func firstForwardedAddress(value string) (netip.Addr, bool) {
	first, _, _ := strings.Cut(value, ",")
	address, err := netip.ParseAddr(strings.TrimSpace(first))
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func isTrusted(address netip.Addr, trusted []netip.Prefix) bool {
	if !address.IsValid() {
		return false
	}
	for _, prefix := range trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
