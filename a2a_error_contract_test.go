package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestA2AHTTPErrorPreservesOwnerContract(t *testing.T) {
	for _, method := range []string{A2AMethodSend, A2AMethodStream} {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-ID", "header-request")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"request","error":{"code":-32005,"message":"权限不足","data":{"reason":"PLUGIN_SETUP_FORBIDDEN"}}}`))
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL, WithAPIKey("test-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			request := AgentA2ARequest{AgentSelector: AgentSelector{AgentID: "agent"}, A2ARequest: A2ARequest{JSONRPC: "2.0", ID: "request", Method: method}}
			if method == A2AMethodStream {
				_, err = client.Agents("workspace").A2AStream(t.Context(), request)
			} else {
				_, err = client.Agents("workspace").A2A(t.Context(), request)
			}
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != "-32005" || failure.Message != "权限不足" || failure.Status != 403 || failure.RequestID != "header-request" {
				t.Fatalf("owner error lost: %#v, %v", failure, err)
			}
			var data map[string]string
			if json.Unmarshal(failure.Data, &data) != nil || data["reason"] != "PLUGIN_SETUP_FORBIDDEN" {
				t.Fatalf("owner data lost: %s", failure.Data)
			}
		})
	}
}

func TestA2AHTTPErrorRejectsMalformedProtocol(t *testing.T) {
	for _, body := range []string{
		`{"jsonrpc":"1.0","id":"x","error":{"code":-32005,"message":"denied"}}`,
		`{"jsonrpc":"2.0","error":{"code":-32005,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":true,"error":{"code":-32005,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":{},"error":{"code":-32005,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":[],"error":{"code":-32005,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":"x","error":{"code":null,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":"x","error":{"code":1.5,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":"x","error":{"code":true,"message":"denied"}}`,
		`{"jsonrpc":"2.0","id":"x","error":{"code":-32005,"message":null}}`,
		`{"jsonrpc":"2.0","id":"x","result":{},"error":{"code":-32005,"message":"denied"}}`,
	} {
		var failure *Error
		if err := parseHTTPError(403, []byte(body), "header-request"); !errors.As(err, &failure) || failure.Code != "ErrResponseInvalid" || failure.Status != 403 || failure.RequestID != "header-request" {
			t.Fatalf("invalid protocol accepted: %s: %v", body, err)
		}
	}
}

func TestProductHTTPErrorWithStructuredDiagnosticPreservesPublicCode(t *testing.T) {
	for _, diagnostic := range []string{
		`{"domain":"semantic_model","reason":"TABLE_NOT_IN_MODEL","metadata":{"table":"payments"}}`,
		`{"code":"diagnostic","message":"not the public message"}`,
	} {
		body := []byte(`{"code":"ErrParamInvalid","msg":"Table is not in the model","error":` + diagnostic + `}`)
		var failure *Error
		err := parseHTTPError(http.StatusBadRequest, body, "request-1")
		if !errors.As(err, &failure) || failure.Code != "ErrParamInvalid" || failure.Message != "Table is not in the model" || failure.Status != 400 || failure.RequestID != "request-1" {
			t.Fatalf("Product envelope confused with nested transport error: %#v, %v", failure, err)
		}
	}
}
