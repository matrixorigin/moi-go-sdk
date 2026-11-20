package sdk

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIngressCallbacksUsePublicTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/newmoi/ingress-callbacks/wecom/ws%2F1/ingress%2F1" {
			t.Errorf("callback path = %s", r.URL.EscapedPath())
		}
		for _, header := range []string{headerAPIKey, headerWorkspaceID, "Authorization", "Cookie"} {
			if r.Header.Get(header) != "" {
				t.Errorf("public callback carried %s", header)
			}
		}
		if r.URL.Query().Get("signature") == "invalid" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"code":6,"message":"signature rejected"}`)
			return
		}
		if r.Header.Get("X-Provider-Signature") != "provider-signature" {
			t.Error("provider signature header was not preserved")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method == http.MethodPost && string(body) != "<xml/>" {
			t.Errorf("provider body = %q, %v", body, err)
		}
		_, _ = io.WriteString(w, "provider-ack")
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("private-key"), WithToken("private-token"), WithSessionCookie("private-cookie"), WithRequestModifier(func(r *http.Request) { r.Header.Set("X-Provider-Signature", "provider-signature") }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	response, err := client.VerifyIngressCallback(t.Context(), "wecom", "ws/1", "ingress/1", WithQueryParam("signature", "valid"))
	if err != nil || string(response) != "provider-ack" {
		t.Fatalf("verify response = %q, %v", response, err)
	}
	response, err = client.PostIngressCallback(t.Context(), "wecom", "ws/1", "ingress/1", []byte("<xml/>"))
	if err != nil || string(response) != "provider-ack" {
		t.Fatalf("event response = %q, %v", response, err)
	}
	_, err = client.PostIngressCallback(t.Context(), "wecom", "ws/1", "ingress/1", []byte("<xml/>"), WithQueryParam("signature", "invalid"))
	if !IsCode(err, "6") || err.(*Error).Message != "signature rejected" {
		t.Fatalf("provider error = %v", err)
	}
}
