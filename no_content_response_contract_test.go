package sdk

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSDKNoContentAcknowledgementKeepsTypedResponsesStrict(t *testing.T) {
	for _, response := range []struct {
		name, body, code string
		status           int
	}{
		{"no_content", "", "", http.StatusNoContent},
		{"empty_success", "", "ErrResponseInvalid", http.StatusOK},
		{"not_found", `{"code":"ErrNotFound","msg":"missing"}`, "ErrNotFound", http.StatusNotFound},
	} {
		t.Run(response.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(response.status)
				_, _ = io.WriteString(w, response.body)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL, WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Error(err)
				}
			})
			value, err := client.Agents("workspace").ResourceDelete(t.Context(), "files", "/file")
			if response.code == "" {
				if err != nil || value == nil || value.Value != nil {
					t.Fatalf("204 acknowledgement invented data or failed: value=%+v err=%v", value, err)
				}
			} else if !IsCode(err, response.code) {
				t.Fatalf("resource response error = %v, want %s", err, response.code)
			}
			_, err = client.Workflows("workspace").List(t.Context())
			wantCode := response.code
			if response.status == http.StatusNoContent {
				wantCode = "ErrResponseInvalid"
			}
			if !IsCode(err, wantCode) {
				t.Fatalf("typed response error = %v, want %s", err, wantCode)
			}
		})
	}
}
