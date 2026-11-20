package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
)

func TestWorkspaceChatCompletionSharedContract(t *testing.T) {
	data, err := os.ReadFile("testdata/workspace_chat_completion_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Input, Expected, Response json.RawMessage
		Query                     map[string]string
		Stream                    string
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	var expected map[string]any
	decoder := json.NewDecoder(bytes.NewReader(fixture.Expected))
	decoder.UseNumber()
	_ = decoder.Decode(&expected)
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/newmoi/workspaces/workspace-1/llm/chat/completions" || r.URL.Query().Get("service_slot") != "slot/1" || r.Header.Get("X-Workspace-ID") != "workspace-1" {
			t.Errorf("request=%s headers=%v", r.URL, r.Header)
		}
		var body map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		_ = decoder.Decode(&body)
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			_, _ = w.Write(fixture.Response)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(fixture.Stream))
	}))
	defer server.Close()
	client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
	if err != nil {
		t.Fatal(err)
	}
	var request WorkspaceChatCompletionRequest
	if err := protojson.Unmarshal(fixture.Input, &request); err != nil {
		t.Fatal(err)
	}
	options := []QueryOption{WithQueryParam("service_slot", "slot/1")}
	result, err := client.Models("workspace-1").WorkspaceChatCompletion(t.Context(), &request, options...)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := result.AsMap(); !ok || value["id"] != "chat-1" {
		t.Fatalf("result=%v", result.Value)
	}
	stream, err := client.Models("workspace-1").WorkspaceChatCompletionStream(t.Context(), &request, options...)
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(stream.Body)
	closeErr := stream.Body.Close()
	if readErr != nil || closeErr != nil || string(content) != fixture.Stream {
		t.Fatalf("stream=%q read=%v close=%v", content, readErr, closeErr)
	}
	streamExpected := maps.Clone(expected)
	streamExpected["stream"] = true
	if !reflect.DeepEqual(bodies[0], expected) || !reflect.DeepEqual(bodies[1], streamExpected) {
		t.Fatalf("bodies=%#v expected=%#v", bodies, []map[string]any{expected, streamExpected})
	}
}
