package sdk

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These public SDK calls distinguish an upstream empty result from a damaged
// response, without changing the server contract or manufacturing data.
func TestNullableSDKResponsesPreserveNullAndRejectMissingData(t *testing.T) {
	for _, response := range []struct {
		name, body, code string
		status           int
	}{
		{"null", `{"code":"OK","msg":"OK","data":null}`, "", 200},
		{"missing", `{"code":"OK","msg":"OK"}`, "ErrResponseInvalid", 200},
		{"empty_body", "", "ErrResponseInvalid", 200},
		{"error_null", `{"code":"ErrConflict","msg":"conflict","data":null}`, "ErrConflict", 409},
		{"error_missing", `{"code":"ErrConflict","msg":"conflict"}`, "ErrConflict", 409},
		{"business_error_200", `{"code":"ErrConflict","msg":"conflict","data":null}`, "ErrConflict", 200},
	} {
		t.Run(response.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-ID", "null-contract")
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
			for _, call := range []struct {
				name   string
				invoke func() error
			}{
				{"query_detail", func() error {
					value, err := client.SQL("workspace").QueryDetail(t.Context(), "statement")
					if err == nil && value != nil {
						t.Error("null query detail manufactured a record")
					}
					return err
				}},
				{"parser_backends", func() error {
					value, err := client.Models("workspace").ParserBackends().List(t.Context())
					if err == nil && len(value.GetBackends()) != 0 {
						t.Error("null backends manufactured entries")
					}
					return err
				}},
				{"parser_endpoints", func() error {
					value, err := client.Models("workspace").ParserBackends().ListEndpoints(t.Context(), "backend")
					if err == nil && len(value.GetEndpoints()) != 0 {
						t.Error("null endpoints manufactured entries")
					}
					return err
				}},
				{"current_credit_requires_object", func() error {
					_, err := client.Account().CurrentCredit(t.Context())
					return err
				}},
			} {
				t.Run(call.name, func(t *testing.T) {
					err := call.invoke()
					wantCode := response.code
					if response.name == "null" && call.name == "current_credit_requires_object" {
						wantCode = "ErrResponseInvalid"
					}
					if wantCode == "" {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					var failure *Error
					if !errors.As(err, &failure) || failure.Code != wantCode || failure.RequestID != "null-contract" {
						t.Fatalf("response classified incorrectly: %v", err)
					}
				})
			}
		})
	}
}
