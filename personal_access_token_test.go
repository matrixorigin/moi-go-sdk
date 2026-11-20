package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewWithPersonalAccessTokenSendsOnlyAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Values("X-API-Key"); len(got) != 1 || got[0] != "opaque-pat" {
			t.Fatalf("X-API-Key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("Cookie"); got != "" {
			t.Fatalf("Cookie = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "msg": "OK", "data": map[string]any{}})
	}))
	defer server.Close()

	client, err := NewWithPersonalAccessToken(server.URL, "opaque-pat")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := client.do(context.Background(), http.MethodGet, "/healthcheck", "", nil, &result); err != nil {
		t.Fatal(err)
	}
}

func TestNewWithPersonalAccessTokenRejectsConflictingCredentials(t *testing.T) {
	client, err := NewWithPersonalAccessToken("https://product.example.test", "opaque-pat", WithToken("bearer-token"))
	if client != nil || err != ErrPersonalAccessTokenAuthConflict {
		t.Fatalf("client=%v err=%v", client, err)
	}
}
