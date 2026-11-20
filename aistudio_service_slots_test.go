package sdk

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIStudioServiceSlotsSDKContract(t *testing.T) {
	type recorded struct{ method, path, query, auth, body string }
	var calls []recorded
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, recorded{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization"), string(body)})
		if r.Header.Get(headerWorkspaceID) != "" {
			t.Errorf("service slot routes are account level, got workspace header %q", r.Header.Get(headerWorkspaceID))
		}
		switch {
		case r.URL.Path == "/newmoi/admin/aistudio/service-slots/forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"ErrForbidden","msg":"需要 taas:admin"}`))
		case r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"slot_id":"chat","default_model_id":"m1","model_ids":[]}`))
		case r.URL.Path == "/newmoi/admin/aistudio/service-slots":
			_, _ = w.Write([]byte(`{"slots":[{"slot_id":"chat"}]}`))
		case r.URL.Path == "/newmoi/admin/aistudio/service-slots/candidates":
			_, _ = w.Write([]byte(`{"models":[{"id":"m1"}]}`))
		default:
			_, _ = w.Write([]byte(`{"logs":[]}`))
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithToken("admin-token"))
	if err != nil {
		t.Fatal(err)
	}
	studio := client.AIStudio()
	ctx := t.Context()

	slots, err := studio.ServiceSlots(ctx)
	if m, ok := slots.AsMap(); err != nil || !ok || m["slots"] == nil {
		t.Fatalf("slots=%v err=%v", slots, err)
	}
	candidates, err := studio.ServiceSlotCandidates(ctx)
	if m, ok := candidates.AsMap(); err != nil || !ok || m["models"] == nil {
		t.Fatalf("candidates=%v err=%v", candidates, err)
	}
	logs, err := studio.ServiceSlotLogs(ctx, WithServiceSlotLogSlotID("chat/1"), WithServiceSlotLogPageSize(5))
	if m, ok := logs.AsMap(); err != nil || !ok || m["logs"] == nil {
		t.Fatalf("logs=%v err=%v", logs, err)
	}
	written, err := studio.PutServiceSlot(ctx, "chat/1", &PutAIStudioServiceSlotRequest{DefaultModelID: "m1"})
	if m, ok := written.AsMap(); err != nil || !ok || m["slot_id"] != "chat" {
		t.Fatalf("written=%v err=%v", written, err)
	}
	if _, err := studio.PutServiceSlot(ctx, "chat", &PutAIStudioServiceSlotRequest{SlotID: "chat", DefaultModelID: "m1", ModelIDs: []string{"m1", "m2"}, ExtraJSON: `{"a":1}`}); err != nil {
		t.Fatal(err)
	}

	want := []recorded{
		{"GET", "/newmoi/admin/aistudio/service-slots", "", "Bearer admin-token", ""},
		{"GET", "/newmoi/admin/aistudio/service-slots/candidates", "", "Bearer admin-token", ""},
		{"GET", "/newmoi/admin/aistudio/service-slots/operation-logs", "page_size=5&slot_id=chat%2F1", "Bearer admin-token", ""},
		{"PUT", "/newmoi/admin/aistudio/service-slots/chat%2F1", "", "Bearer admin-token", `{"default_model_id":"m1","model_ids":[],"extra_json":""}`},
		{"PUT", "/newmoi/admin/aistudio/service-slots/chat", "", "Bearer admin-token", `{"slot_id":"chat","default_model_id":"m1","model_ids":["m1","m2"],"extra_json":"{\"a\":1}"}`},
	}
	if len(calls) != len(want) {
		t.Fatalf("calls=%v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d=%+v want %+v", i, calls[i], want[i])
		}
	}
	if !json.Valid([]byte(calls[4].body)) {
		t.Errorf("invalid body %q", calls[4].body)
	}

	_, err = studio.PutServiceSlot(ctx, "forbidden", &PutAIStudioServiceSlotRequest{DefaultModelID: "m1"})
	var sdkErr *Error
	if !errors.As(err, &sdkErr) || sdkErr.Code != "ErrForbidden" || sdkErr.Message != "需要 taas:admin" {
		t.Fatalf("error=%#v", err)
	}
	if _, err := studio.PutServiceSlot(ctx, "", &PutAIStudioServiceSlotRequest{}); !errors.Is(err, ErrIDRequired) {
		t.Fatalf("empty slot id err=%v", err)
	}
	if _, err := studio.PutServiceSlot(ctx, "chat", nil); err == nil {
		t.Fatal("nil request must fail")
	}
}

func TestAIStudioChatModelPolicyPreservesAstraContract(t *testing.T) {
	for _, auth := range []Option{WithToken("ordinary-token"), WithAPIKey("runtime-pat")} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/newmoi/aistudio/model-policy" || r.Header.Get(headerWorkspaceID) != "" {
				t.Errorf("unexpected account-level policy request: %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer ordinary-token" && r.Header.Get("X-API-Key") != "runtime-pat" {
				t.Error("model policy request lost product authentication")
			}
			_, _ = io.WriteString(w, `{"code":"OK","data":{"default_model":"second","models":["second","first"]}}`)
		}))
		client, err := New(server.URL+"/newmoi", auth)
		if err != nil {
			t.Fatal(err)
		}
		policy, err := client.AIStudio().ChatModelPolicy(t.Context())
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		data, ok := policy.AsMap()
		if !ok || data["default_model"] != "second" {
			t.Fatalf("policy=%v", policy)
		}
	}
}
