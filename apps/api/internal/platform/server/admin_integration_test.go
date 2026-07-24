//go:build integration

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/admin"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/tokens"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

func TestAdministrationHTTPAuthorizationAndRecentAuthentication(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, runtimeConfig.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	now := time.Date(2026, 7, 24, 19, 0, 0, 0, time.UTC)
	timeSource := clock.NewFake(now)
	idSource := ids.NewUUIDv7()
	authConfig := config.Auth{
		Issuer: "https://phase6.example", Audience: "phase6-web", ActiveKeyID: "phase6",
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour,
		RecentAuthTTL: 10 * time.Minute,
	}
	ring, err := tokens.NewEphemeral(authConfig, timeSource)
	if err != nil {
		t.Fatal(err)
	}
	sessionService := sessions.New(pool, idSource, timeSource, ring, time.Hour)
	cookieConfig := config.Cookies{
		SigningKey: []byte("phase6-cookie-signing-key-32bytes"),
		SameSite:   "lax",
	}
	browserManager := browser.New(cookieConfig, authConfig.AccessTokenTTL, authConfig.RefreshTokenTTL)
	adminID, userID, targetID := uuid.New(), uuid.New(), uuid.New()
	suffix := uuid.NewString()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO users (id, email, display_name, role)
		VALUES ($1, $2, 'HTTP Admin', 'admin'),
		       ($3, $4, 'HTTP User', 'user'),
		       ($5, $6, 'HTTP Target', 'user')
	`, adminID, "phase6-http-admin-"+suffix+"@example.com",
		userID, "phase6-http-user-"+suffix+"@example.com",
		targetID, "phase6-http-target-"+suffix+"@example.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(),
			`DELETE FROM users WHERE id = ANY($1)`, []uuid.UUID{adminID, userID, targetID})
	})

	handler := New(Dependencies{
		Config: config.Config{
			Runtime: config.Runtime{
				MaxBodyBytes: 1 << 20, RequestTimeout: 5 * time.Second,
				AllowedOrigins: []string{"http://localhost:3000"},
			},
			Auth: authConfig,
		},
		Database: pool, Sessions: sessionService, Browser: browserManager,
		Auth: browser.Authenticate(ring, sessionService), IDs: idSource, Clock: timeSource,
		Admin: admin.New(pool, idSource, timeSource),
	})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	anonymous := request(t, http.DefaultClient, http.MethodGet,
		httpServer.URL+"/api/v1/admin/users", nil, nil)
	assertStatus(t, anonymous, http.StatusUnauthorized)

	regularClient, _, _ := authenticatedAdminClient(
		t, ctx, httpServer.URL, browserManager, sessionService, userID,
	)
	forbidden := request(t, regularClient, http.MethodGet,
		httpServer.URL+"/api/v1/admin/users", nil, nil)
	assertStatus(t, forbidden, http.StatusForbidden)

	adminClient, csrf, sessionID := authenticatedAdminClient(
		t, ctx, httpServer.URL, browserManager, sessionService, adminID,
	)
	list := request(t, adminClient, http.MethodGet,
		httpServer.URL+"/api/v1/admin/users?query="+suffix+"&limit=1", nil, nil)
	assertStatus(t, list, http.StatusOK)

	if _, err := pool.Raw().Exec(ctx,
		`UPDATE auth_sessions SET recent_auth_at = $1 WHERE id = $2`,
		now.Add(-authConfig.RecentAuthTTL-time.Minute), sessionID,
	); err != nil {
		t.Fatal(err)
	}
	roleURL := httpServer.URL + "/api/v1/admin/users/" + targetID.String() + "/role"
	mutationHeaders := map[string]string{
		"X-CSRF-Token": csrf,
		"If-Match":     `"1"`,
	}
	staleRecent := request(
		t, adminClient, http.MethodPatch, roleURL,
		[]byte(`{"role":"admin"}`), mutationHeaders,
	)
	assertProblem(t, staleRecent, http.StatusPreconditionRequired, "recent_auth_required")

	if _, err := pool.Raw().Exec(ctx,
		`UPDATE auth_sessions SET recent_auth_at = $1 WHERE id = $2`, now, sessionID,
	); err != nil {
		t.Fatal(err)
	}
	missingCSRF := request(
		t, adminClient, http.MethodPatch, roleURL,
		[]byte(`{"role":"admin"}`), map[string]string{"If-Match": `"1"`},
	)
	assertProblem(t, missingCSRF, http.StatusForbidden, "forbidden")

	updated := request(
		t, adminClient, http.MethodPatch, roleURL,
		[]byte(`{"role":"admin"}`), mutationHeaders,
	)
	assertStatus(t, updated, http.StatusOK)
	var updatedUser admin.User
	decodeResponse(t, updated, &updatedUser)
	if updatedUser.Role != "admin" || updatedUser.Version != 2 {
		t.Fatalf("updated user = %#v", updatedUser)
	}

	conflict := request(
		t, adminClient, http.MethodPatch, roleURL,
		[]byte(`{"role":"user"}`), mutationHeaders,
	)
	assertProblem(t, conflict, http.StatusConflict, "conflict")

	if _, err := pool.Raw().Exec(ctx,
		`UPDATE auth_sessions SET recent_auth_at = $1 WHERE id = $2`,
		now.Add(time.Minute), sessionID,
	); err != nil {
		t.Fatal(err)
	}
	futureRecent := request(
		t, adminClient, http.MethodPatch, roleURL,
		[]byte(`{"role":"user"}`),
		map[string]string{"X-CSRF-Token": csrf, "If-Match": `"2"`},
	)
	assertProblem(t, futureRecent, http.StatusPreconditionRequired, "recent_auth_required")

	invalidUsage := request(t, adminClient, http.MethodGet,
		httpServer.URL+"/api/v1/admin/usage?from=invalid", nil, nil)
	assertProblem(t, invalidUsage, http.StatusBadRequest, "invalid_request")
}

func authenticatedAdminClient(
	t *testing.T,
	ctx context.Context,
	serverURL string,
	manager *browser.Manager,
	service *sessions.Service,
	userID uuid.UUID,
) (*http.Client, string, uuid.UUID) {
	t.Helper()
	credentials, err := service.Create(ctx, userID, "phase6-http")
	if err != nil {
		t.Fatal(err)
	}
	issued := httptest.NewRecorder()
	csrf, err := manager.Issue(issued, credentials)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := mustURL(t, serverURL)
	jar.SetCookies(endpoint, issued.Result().Cookies())
	return &http.Client{Jar: jar}, csrf, credentials.SessionID
}

func mustURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func assertProblem(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	if response.StatusCode != status {
		assertStatus(t, response, status)
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != code {
		t.Fatalf("problem code = %q, want %q", payload.Error.Code, code)
	}
}
