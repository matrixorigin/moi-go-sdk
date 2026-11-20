package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestConversationMatchesSharedHTTPContract(t *testing.T) {
	var cases []struct {
		Name, Operation, Method, Path string
		SessionID                     int64 `json:"session_id"`
		MessageID                     int64 `json:"message_id"`
		Input, Request, Response      json.RawMessage
		Status                        int
	}
	data, err := os.ReadFile("testdata/conversation_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			decodeJSON := func(data []byte) any {
				t.Helper()
				var value any
				decoder := json.NewDecoder(bytes.NewReader(data))
				decoder.UseNumber()
				if err := decoder.Decode(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			received := make(chan any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantContentType := ""
				if len(scenario.Input) != 0 {
					wantContentType = "application/json"
				}
				if r.Method != scenario.Method || r.URL.RequestURI() != scenario.Path || r.Header.Get("X-API-Key") != "synthetic-key" || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("Content-Type") != wantContentType {
					t.Errorf("unexpected scoped request: %s %s", r.Method, r.URL.RequestURI())
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				var body any
				if len(raw) != 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if err := decoder.Decode(&body); err != nil {
						t.Error(err)
					}
				}
				received <- body
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				if scenario.Status != http.StatusNoContent {
					_, _ = w.Write(scenario.Response)
				}
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("synthetic-key"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			service := client.Conversations("workspace-1")
			decode := func(request proto.Message) {
				t.Helper()
				if err := json.Unmarshal(scenario.Input, request); err != nil {
					t.Fatal(err)
				}
			}
			var result any
			var callErr error
			switch scenario.Operation {
			case "create-ui-session", "update-ui-session":
				request := &ConversationUISessionRequest{}
				decode(request)
				if scenario.Operation == "create-ui-session" {
					result, callErr = service.CreateUISession(t.Context(), request)
				} else {
					result, callErr = service.UpdateUISession(t.Context(), scenario.SessionID, request)
				}
			case "list-ui-sessions", "list-pinned-ui-sessions", "list-unpinned-ui-sessions":
				request := &ConversationUISessionListRequest{}
				decode(request)
				switch scenario.Operation {
				case "list-ui-sessions":
					result, callErr = service.ListUISessions(t.Context(), request)
				case "list-pinned-ui-sessions":
					result, callErr = service.ListPinnedUISessions(t.Context(), request)
				case "list-unpinned-ui-sessions":
					result, callErr = service.ListUnpinnedUISessions(t.Context(), request)
				}
			case "list-ui-messages":
				request := &ConversationUIMessageListRequest{}
				decode(request)
				result, callErr = service.ListUIMessages(t.Context(), request)
			case "create-session", "update-session":
				request := &ConversationWorkspaceSessionRequest{}
				decode(request)
				if scenario.Operation == "create-session" {
					result, callErr = service.CreateSession(t.Context(), request)
				} else {
					result, callErr = service.UpdateSession(t.Context(), scenario.SessionID, request)
				}
			case "create-message":
				request := &ConversationWorkspaceMessageRequest{}
				decode(request)
				result, callErr = service.CreateMessage(t.Context(), scenario.SessionID, request)
			case "create-tag", "add-session-tag", "add-message-tag":
				request := &ConversationTagRequest{}
				decode(request)
				switch scenario.Operation {
				case "create-tag":
					result, callErr = service.CreateTag(t.Context(), request)
				case "add-session-tag":
					callErr = service.AddSessionTag(t.Context(), scenario.SessionID, request)
				case "add-message-tag":
					callErr = service.AddMessageTag(t.Context(), scenario.MessageID, request)
				}
			case "get-ui-session":
				result, callErr = service.GetUISession(t.Context(), scenario.SessionID)
			case "delete-ui-session":
				result, callErr = service.DeleteUISession(t.Context(), scenario.SessionID)
			case "pin-ui-session":
				result, callErr = service.PinUISession(t.Context(), scenario.SessionID)
			case "unpin-ui-session":
				result, callErr = service.UnpinUISession(t.Context(), scenario.SessionID)
			case "get-ui-message":
				result, callErr = service.GetUIMessage(t.Context(), scenario.MessageID)
			case "like-ui-message":
				result, callErr = service.LikeUIMessage(t.Context(), scenario.MessageID)
			case "dislike-ui-message":
				result, callErr = service.DislikeUIMessage(t.Context(), scenario.MessageID)
			case "latest-ui-message-id":
				result, callErr = service.LatestUIMessageID(t.Context(), scenario.SessionID)
			default:
				t.Fatalf("unsupported fixture operation %s", scenario.Operation)
			}
			select {
			case body := <-received:
				if want := decodeJSON(scenario.Request); !reflect.DeepEqual(body, want) {
					t.Errorf("HTTP body=%v, want=%v", body, want)
				}
			default:
				t.Fatal("no HTTP request received")
			}
			var response struct {
				Code, Msg string
				Data      json.RawMessage
			}
			if scenario.Status >= 400 {
				if err := json.Unmarshal(scenario.Response, &response); err != nil {
					t.Fatal(err)
				}
				var failure *Error
				if !errors.As(callErr, &failure) || failure.Code != response.Code || failure.Status != scenario.Status || failure.Message != response.Msg {
					t.Fatalf("owner failure changed: %v", callErr)
				}
				return
			}
			if callErr != nil {
				t.Fatal(callErr)
			}
			if scenario.Status == http.StatusNoContent {
				return
			}
			var envelope struct{ Data json.RawMessage }
			if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
				t.Fatal(err)
			}
			check := func(actual proto.Message, raw []byte) {
				t.Helper()
				want := actual.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(raw, want); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(actual, want) {
					t.Errorf("response=%v, want=%v", actual, want)
				}
			}
			if scenario.Operation == "list-ui-messages" {
				var want []json.RawMessage
				if err := json.Unmarshal(envelope.Data, &want); err != nil {
					t.Fatal(err)
				}
				actual := result.([]*ConversationUIMessage)
				if len(actual) != len(want) {
					t.Fatalf("message response=%v, want=%s", actual, envelope.Data)
				}
				for i, message := range actual {
					check(message, want[i])
				}
			} else {
				check(result.(proto.Message), envelope.Data)
			}
		})
	}
}
