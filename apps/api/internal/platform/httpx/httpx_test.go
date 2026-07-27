package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

func TestMiddlewareHeadersAndTrustedProxy(t *testing.T) {
	source := ids.NewFake(uuid.MustParse("018f0000-0000-7000-8000-000000000001"))
	handler := RequestIDs(source)(
		SecurityHeaders(
			ClientIPs([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})(
				http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
					WriteJSON(response, http.StatusOK, map[string]string{"client": ClientIP(request.Context()).String()})
				}),
			),
		),
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.5")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if !strings.Contains(response.Body.String(), "203.0.113.9") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler := RequestIDs(ids.NewFake(uuid.MustParse("018f0000-0000-7000-8000-000000000001")))(
		CORS([]string{"https://glazz.example"})(
			http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(http.StatusNoContent)
			}),
		),
	)
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestCORSPreflightAllowsConditionalMutations(t *testing.T) {
	handler := CORS([]string{"https://glazz.example"})(
		http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusNoContent)
		}),
	)
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://glazz.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set(
		"Access-Control-Request-Headers",
		"content-type,if-match,x-csrf-token",
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	allowedHeaders := response.Header().Get("Access-Control-Allow-Headers")
	for _, expected := range []string{"Content-Type", "If-Match", "X-CSRF-Token"} {
		if !strings.Contains(allowedHeaders, expected) {
			t.Fatalf("Access-Control-Allow-Headers = %q, missing %q", allowedHeaders, expected)
		}
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPatch) {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
}

func TestRecoveryDoesNotEchoPanicValue(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := RequestIDs(ids.NewFake(uuid.MustParse("018f0000-0000-7000-8000-000000000001")))(
		Recovery(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("sensitive panic payload")
		})),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "sensitive") {
		t.Fatalf("response leaked panic value: %s", response.Body.String())
	}
}
