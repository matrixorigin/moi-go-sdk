package sdk

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type accountNativeHTTPContract struct {
	Name     string          `json:"name"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Status   int             `json:"status"`
}

func TestAccountNativeSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/account_native_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []accountNativeHTTPContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || r.Header.Get("Authorization") != "Bearer native-token" || r.Header.Get("X-API-Key") != "" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("Cookie") != "" || len(raw) != 0 {
					t.Errorf("request=%s %s body=%q headers=%v", r.Method, r.URL.RequestURI(), raw, r.Header)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithToken("native-token"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Account().BootstrapNative(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
				t.Fatal(err)
			}
			expected := &NativeBootstrapResult{}
			if err := protojson.Unmarshal(envelope.Data, expected); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(result, expected) {
				t.Fatalf("result=%v want=%v", result, expected)
			}
		})
	}
}
