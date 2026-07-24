package browser

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

func testCookieConfig(key string) config.Cookies {
	return config.Cookies{SigningKey: []byte(key), SameSite: "lax"}
}

func TestIssueUsesSecureBrowserCookiePolicy(t *testing.T) {
	manager := New(config.Cookies{
		SigningKey: []byte("01234567890123456789012345678901"),
		Domain:     "glazz.example",
		Secure:     true,
		SameSite:   "lax",
	}, 15*time.Minute, 30*24*time.Hour)
	response := httptest.NewRecorder()
	if _, err := manager.Issue(response, sessions.Credentials{
		AccessToken: "access", RefreshToken: "refresh",
	}); err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	for _, cookie := range cookies {
		if !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode ||
			cookie.Path != "/" || cookie.Domain != "glazz.example" {
			t.Fatalf("cookie policy = %+v", cookie)
		}
	}
}

func TestCSRFRejectsMissingAndAcceptsSignedDoubleSubmit(t *testing.T) {
	manager := New(config.Cookies{
		SigningKey: []byte("01234567890123456789012345678901"),
		SameSite:   "lax",
	}, time.Minute, time.Hour)
	handler := manager.CSRF(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, "/", nil))
	if missing.Code != http.StatusForbidden {
		t.Fatalf("missing status = %d", missing.Code)
	}

	token := manager.sign("csrf-token")
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(&http.Cookie{Name: CSRFCookie, Value: token})
	request.Header.Set("X-CSRF-Token", token)
	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, request)
	if valid.Code != http.StatusNoContent {
		t.Fatalf("valid status = %d", valid.Code)
	}
}

func TestRefreshCSRFClearsCookiesSignedWithPreviousKey(t *testing.T) {
	previous := New(testCookieConfig("previous-signing-key-32-bytes!!"), time.Minute, time.Hour)
	current := New(testCookieConfig("current-signing-key--32-bytes!!"), time.Minute, time.Hour)
	credentials := sessions.Credentials{AccessToken: "access", RefreshToken: "refresh"}
	issued := httptest.NewRecorder()
	csrf, err := previous.Issue(issued, credentials)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	for _, cookie := range issued.Result().Cookies() {
		request.AddCookie(cookie)
	}
	request.Header.Set("X-CSRF-Token", csrf)
	response := httptest.NewRecorder()

	current.RefreshCSRF(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not run with a previous signing key")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	cleared := response.Result().Cookies()
	if len(cleared) != 3 {
		t.Fatalf("cleared cookies = %d, want 3", len(cleared))
	}
	for _, cookie := range cleared {
		if cookie.MaxAge >= 0 {
			t.Errorf("%s MaxAge = %d, want negative", cookie.Name, cookie.MaxAge)
		}
	}
}
