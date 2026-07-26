package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

func TestLiveness(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestMaintenanceRejectsNewGuestSession(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/guest-sessions", nil)
	response := httptest.NewRecorder()
	Dependencies{Config: config.Config{
		Runtime: config.Runtime{Maintenance: true},
	}}.createOrResumeGuest(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestOAuthTestAuthorizationEscapesState(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/auth/test/authorize?state=%22%3E%3Cscript%3Ealert(1)%3C/script%3E",
		nil,
	)
	response := httptest.NewRecorder()
	Dependencies{Config: config.Config{
		OAuth: config.OAuth{TestMode: true, TestEmail: "e2e@example.com"},
	}}.testAuthorize(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if strings.Contains(response.Body.String(), "<script>") {
		t.Fatal("authorization page contains unescaped state")
	}
}
