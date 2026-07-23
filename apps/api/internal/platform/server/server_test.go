package server

import (
	"net/http"
	"net/http/httptest"
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
