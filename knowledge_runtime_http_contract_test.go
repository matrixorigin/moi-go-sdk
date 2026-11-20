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

func TestKnowledgeRuntimeMatchesSharedHTTPContract(t *testing.T) {
	var cases []struct {
		Name, Operation          string
		Input, Request, Response json.RawMessage
		Status                   int
	}
	data, err := os.ReadFile("testdata/knowledge_runtime_http_contract.json")
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
				if r.Method != "POST" || r.URL.Path != "/newmoi/semantic-models/runtime/"+scenario.Operation || r.URL.RawQuery != "" || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("X-MOI-Runtime-Authorization") != "synthetic-live-task" {
					t.Errorf("unexpected scoped request: %s %s", r.Method, r.URL.RequestURI())
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
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"), WithRuntimeAuthorization("synthetic-live-task"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			service := client.Knowledge("workspace-1")
			var request proto.Message
			var invoke func() (*KnowledgeRuntimeResult, error)
			switch scenario.Operation {
			case "find-files":
				value := &KnowledgeFindFilesRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.FindFiles(t.Context(), value) }
			case "search":
				value := &KnowledgeSearchRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.Search(t.Context(), value) }
			case "read-document":
				value := &KnowledgeReadDocumentRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.ReadDocument(t.Context(), value) }
			case "search-document":
				value := &KnowledgeSearchDocumentRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.SearchDocument(t.Context(), value) }
			case "search-images":
				value := &KnowledgeSearchImagesRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.SearchImages(t.Context(), value) }
			case "read-evidence":
				value := &KnowledgeReadEvidenceRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.ReadEvidence(t.Context(), value) }
			case "describe-tables":
				value := &KnowledgeDescribeTablesRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.DescribeTables(t.Context(), value) }
			case "query":
				value := &KnowledgeQueryRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.Query(t.Context(), value) }
			case "cite":
				value := &KnowledgeCiteRequest{}
				request = value
				invoke = func() (*KnowledgeRuntimeResult, error) { return service.Cite(t.Context(), value) }
			default:
				t.Fatalf("unknown fixture operation %s", scenario.Operation)
			}
			if err := json.Unmarshal(scenario.Input, request); err != nil {
				t.Fatal(err)
			}
			result, err := invoke()
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
					t.Fatalf("owner failure changed: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var expected KnowledgeRuntimeResult
			if err := protojson.Unmarshal(response.Data, &expected); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(result, &expected) {
				t.Errorf("result=%v, want=%v", result, &expected)
			}
		})
	}
}
