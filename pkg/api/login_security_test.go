package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PLAT-499: login must not put the password in the request URL, and the
// default client must not install a stderr logger that would print URLs.
func TestLogin_PasswordNotInURL(t *testing.T) {
	const secret = "super-secret-password-not-for-url"
	var sawURL, sawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawURL = r.URL.String()
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		if r.Method != http.MethodPost {
			t.Errorf("login method = %s, want POST", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"sid":       "test-sid",
				"synotoken": "test-token",
			},
		})
	}))
	defer srv.Close()

	c, err := New(Options{Host: srv.URL, AllowHTTP: true, VerifyCert: false})
	if err != nil {
		t.Fatal(err)
	}
	// Ensure default logger is off (retryablehttp would otherwise log URLs).
	if c.Client().Logger != nil {
		t.Fatal("default client Logger must be nil so URLs are not written to stderr")
	}

	_, err = c.Login(context.Background(), LoginOptions{
		Username: "admin",
		Password: secret,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if strings.Contains(sawURL, secret) || strings.Contains(sawURL, "passwd=") {
		t.Fatalf("password leaked into request URL: %s", sawURL)
	}
	if !strings.Contains(sawBody, "passwd=") {
		t.Fatalf("expected passwd in POST body, body=%q", sawBody)
	}
	if !strings.Contains(sawBody, secret) {
		t.Fatalf("expected password value in POST body, body=%q", sawBody)
	}
	if !strings.Contains(sawBody, "account=admin") && !strings.Contains(sawBody, "account=") {
		t.Fatalf("expected account in body, body=%q", sawBody)
	}
}
