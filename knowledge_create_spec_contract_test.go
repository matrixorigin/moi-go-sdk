package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestKnowledgeCreateSpecMatchesSharedHandlerContract(t *testing.T) {
	var cases []struct {
		Name              string
		Spec              SemanticModelCreateSpec
		Options           map[string]json.RawMessage
		Request, Response json.RawMessage
		Status            int
	}
	data, err := os.ReadFile("testdata/knowledge_create_spec_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			var want any
			decoder := json.NewDecoder(bytes.NewReader(scenario.Request))
			decoder.UseNumber()
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			received := make(chan any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/newmoi/semantic-models" || r.URL.RawQuery != "" || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
				}
				var got any
				decoder := json.NewDecoder(r.Body)
				decoder.UseNumber()
				if err := decoder.Decode(&got); err != nil {
					t.Error(err)
				}
				received <- got
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			var opts []SemanticModelOption
			for field, value := range scenario.Options {
				switch field {
				case "description":
					var text string
					if err := json.Unmarshal(value, &text); err != nil {
						t.Fatal(err)
					}
					opts = append(opts, WithSemanticModelDescription(text))
				case "tables":
					opts = append(opts, WithSemanticModelTables(value))
				case "files":
					opts = append(opts, WithSemanticModelFiles(value))
				}
			}
			result, err := client.Knowledge("workspace-1").CreateWithSpec(t.Context(), scenario.Spec, opts...)
			select {
			case got := <-received:
				if !reflect.DeepEqual(got, want) {
					t.Errorf("request=%v, want=%v", got, want)
				}
			default:
				t.Fatal("no HTTP request received")
			}
			var response struct {
				Code, Msg string
				Data      json.RawMessage
			}
			if decodeErr := json.Unmarshal(scenario.Response, &response); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if scenario.Status >= 400 {
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != response.Code || failure.Message != response.Msg || failure.Status != scenario.Status {
					t.Fatalf("owner rejection changed: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var expected SemanticModelInfo
			if err := protojson.Unmarshal(response.Data, &expected); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(result, &expected) {
				t.Errorf("response=%v, want=%v", result, &expected)
			}
		})
	}
}
