//go:build integration

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/guests"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

func TestConversationHTTPContractForGuest(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 6, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	signingKey := []byte("01234567890123456789012345678901")
	cookies := config.Cookies{SigningKey: signingKey, SameSite: "lax"}
	timeSource := clock.UTC{}
	idSource := ids.NewUUIDv7()
	guestService := guests.New(pool, idSource, timeSource, cookies, time.Hour)
	conversationService := conversations.New(pool, models.New(pool), idSource, timeSource)
	handler := New(Dependencies{
		Config: config.Config{Runtime: config.Runtime{
			MaxBodyBytes: 1 << 20, RequestTimeout: 5 * time.Second,
			AllowedOrigins: []string{"http://localhost:3000"},
		}},
		Database: pool,
		Guests:   guestService,
		Browser:  browser.New(cookies, 15*time.Minute, time.Hour),
		Auth: func(next http.Handler) http.Handler {
			return next
		},
		IDs:    idSource,
		Models: models.New(pool),
		Chats:  conversationService,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	response := request(t, client, http.MethodGet, server.URL+"/api/v1/conversations", nil, nil)
	assertStatus(t, response, http.StatusUnauthorized)

	response = request(t, client, http.MethodPost, server.URL+"/api/v1/guest-sessions", nil, nil)
	assertStatus(t, response, http.StatusCreated)
	csrf := cookieValue(client, response.Request.URL, guests.CSRFCookieName)
	if csrf == "" {
		t.Fatal("guest CSRF cookie was not issued")
	}
	mutationHeaders := map[string]string{"X-CSRF-Token": csrf}

	response = request(
		t, client, http.MethodPost, server.URL+"/api/v1/conversations",
		[]byte(`{"title":"HTTP contract"}`), mutationHeaders,
	)
	assertStatus(t, response, http.StatusBadRequest)

	createHeaders := map[string]string{
		"X-CSRF-Token": csrf, "Idempotency-Key": "http-create-conversation-0001",
	}
	response = request(
		t, client, http.MethodPost, server.URL+"/api/v1/conversations",
		[]byte(`{"title":"HTTP contract"}`), createHeaders,
	)
	assertStatus(t, response, http.StatusCreated)
	var created conversations.Conversation
	decodeResponse(t, response, &created)
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(), `
			DELETE FROM guest_sessions
			WHERE id = (SELECT guest_session_id FROM conversations WHERE id = $1)
		`, created.ID)
	})

	response = request(
		t, client, http.MethodPost, server.URL+"/api/v1/conversations",
		[]byte(`{"title":"Different replay payload"}`), createHeaders,
	)
	assertStatus(t, response, http.StatusCreated)
	var replay conversations.Conversation
	decodeResponse(t, response, &replay)
	if replay.ID != created.ID || replay.Title != created.Title {
		t.Fatalf("idempotent replay = %#v, created = %#v", replay, created)
	}

	resourceURL := server.URL + "/api/v1/conversations/" + created.ID.String()
	response = request(t, client, http.MethodGet, resourceURL, nil, nil)
	assertStatus(t, response, http.StatusOK)
	etag := response.Header.Get("ETag")
	if etag == "" {
		t.Fatal("conversation response omitted ETag")
	}
	response.Body.Close()
	response = request(t, client, http.MethodGet, resourceURL, nil, map[string]string{
		"If-None-Match": etag,
	})
	assertStatus(t, response, http.StatusNotModified)

	response = request(
		t, client, http.MethodPatch, resourceURL,
		[]byte(`{"unknown":true}`), mutationHeaders,
	)
	assertStatus(t, response, http.StatusBadRequest)
	response = request(
		t, client, http.MethodPatch, resourceURL,
		[]byte(`{"archived":true}`), mutationHeaders,
	)
	assertStatus(t, response, http.StatusForbidden)
	response = request(
		t, client, http.MethodGet,
		server.URL+"/api/v1/conversations/"+uuid.NewString(), nil, nil,
	)
	assertStatus(t, response, http.StatusNotFound)
	response = request(
		t, client, http.MethodGet, resourceURL+"/messages?after=invalid", nil, nil,
	)
	assertStatus(t, response, http.StatusBadRequest)

	response = request(t, client, http.MethodGet, server.URL+"/api/v1/conversations", nil, nil)
	assertStatus(t, response, http.StatusOK)
	if response.Header.Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("list Cache-Control = %q", response.Header.Get("Cache-Control"))
	}
	response.Body.Close()

	deleteHeaders := map[string]string{
		"X-CSRF-Token": csrf, "Idempotency-Key": "http-delete-conversation-0001",
	}
	response = request(t, client, http.MethodDelete, resourceURL, nil, deleteHeaders)
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, client, http.MethodDelete, resourceURL, nil, deleteHeaders)
	assertStatus(t, response, http.StatusNoContent)
	response = request(t, client, http.MethodDelete, resourceURL, nil, map[string]string{
		"X-CSRF-Token": csrf, "Idempotency-Key": "http-delete-conversation-0002",
	})
	assertStatus(t, response, http.StatusNotFound)
}

func request(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	body []byte,
	headers map[string]string,
) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func assertStatus(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		var payload any
		_ = json.NewDecoder(response.Body).Decode(&payload)
		t.Fatalf("status = %d, want %d, body = %#v", response.StatusCode, expected, payload)
	}
}

func decodeResponse(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func cookieValue(client *http.Client, endpoint *url.URL, name string) string {
	for _, cookie := range client.Jar.Cookies(endpoint) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}
