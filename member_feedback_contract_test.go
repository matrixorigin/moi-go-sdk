package sdk

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestMemberInvitationAndFeedbackRewardSDKContracts(t *testing.T) {
	for _, auth := range []struct {
		name, header, value string
		option              Option
	}{
		{"api-key", "X-API-Key", "target-key", WithAPIKey("target-key")},
		{"bearer", "Authorization", "Bearer target-token", WithToken("target-token")},
	} {
		t.Run(auth.name, func(t *testing.T) {
			var calls []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.EscapedPath())
				if r.Header.Get(auth.header) != auth.value || r.Header.Get(headerWorkspaceID) != "workspace-1" {
					t.Errorf("request headers=%v", r.Header)
				}
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`{"code":"OK","data":{"id":"ticket-1","submitter_email":"target@example.test"}}`))
					return
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.URL.Path == "/newmoi/workspaces/invitations/invitation-1/accept" {
					if len(body) != 0 {
						t.Errorf("accept body=%v, want empty object", body)
					}
					_, _ = w.Write([]byte(`{"code":"OK","data":{"accepted":true}}`))
					return
				}
			if len(body) != 2 || body["status"] != "closed" || body["admin_reply"] != "" {
					t.Errorf("approval body=%v, want explicit status and empty reply", body)
				}
				_, _ = w.Write([]byte(`{"code":"OK","data":{"ticket":{"id":"ticket-1"},"reward_event":{"id":"event-1"}}}`))
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", auth.option)
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := client.WorkspaceMembers("workspace-1").AcceptInvitation(t.Context(), "invitation-1")
			if err != nil || !accepted.GetAccepted() {
				t.Fatalf("accept=%v err=%v", accepted, err)
			}
			ticket, err := client.Agents("workspace-1").AdminFeedbackTicket(t.Context(), "ticket-1")
			if err != nil {
				t.Fatal(err)
			}
			data, ok := ticket.AsMap()
			if !ok || data["submitter_email"] != "target@example.test" {
				t.Fatalf("admin projection=%v err=%v", ticket, err)
			}
			status, reply := "closed", ""
			approval, err := client.Agents("workspace-1").ApproveFeedbackReward(t.Context(), "ticket-1", &FeedbackTicketAdminUpdateRequest{Status: &status, AdminReply: &reply})
			if err != nil {
				t.Fatal(err)
			}
			data, ok = approval.AsMap()
			if !ok || data["reward_event"] == nil {
				t.Fatalf("approval=%v err=%v", approval, err)
			}
			want := []string{"POST /newmoi/workspaces/invitations/invitation-1/accept", "GET /newmoi/admin/feedback-tickets/ticket-1", "POST /newmoi/admin/feedback-tickets/ticket-1/reward-approval"}
			if !slices.Equal(calls, want) {
				t.Fatalf("calls=%v, want %v", calls, want)
			}
		})
	}
}

func TestInvitationAcceptSDKRejectsUnknownReceiptFields(t *testing.T) {
	var result WorkspaceInvitationAcceptResult
	if err := parseProductResponse([]byte(`{"code":"OK","data":{"accepted":true,"unexpected":true}}`), &result, ""); err == nil {
		t.Fatal("invalid invitation acceptance response accepted")
	}
}

func TestFeedbackGitHubIssueSDKRoutes(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath())
		if r.Header.Get("Authorization") != "Bearer admin-token" || r.Header.Get(headerWorkspaceID) != "workspace-1" {
			t.Errorf("request headers=%v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(calls) == 1 {
			if len(body) != 1 || body["kind"] != "bug" {
				t.Errorf("draft body=%v", body)
			}
			_, _ = w.Write([]byte(`{"code":"OK","data":{"kind":"bug","title":"draft"}}`))
			return
		}
		if len(body) != 4 || body["kind"] != "bug" || body["title"] != "draft" || body["body"] != "details" || body["retry"] != true {
			t.Errorf("create body=%v", body)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"ErrFeedbackGitHubIssueCreatorUnavailable","msg":"创建器不可用"}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithToken("admin-token"))
	if err != nil {
		t.Fatal(err)
	}
	draft, err := client.Agents("workspace-1").BuildFeedbackGithubIssueDraft(t.Context(), "ticket-1", "bug")
	if err != nil {
		t.Fatal(err)
	}
	data, ok := draft.AsMap()
	if !ok || data["title"] != "draft" {
		t.Fatalf("draft=%v", draft)
	}
	_, err = client.Agents("workspace-1").CreateFeedbackGithubIssue(t.Context(), "ticket-1", "bug", "draft", "details", true)
	var failure *Error
	if !errors.As(err, &failure) || failure.Status != http.StatusServiceUnavailable || failure.Code != "ErrFeedbackGitHubIssueCreatorUnavailable" || failure.Message != "创建器不可用" {
		t.Fatalf("public error=%v", err)
	}
	want := []string{"POST /newmoi/admin/feedback-tickets/ticket-1/github-issue-draft", "POST /newmoi/admin/feedback-tickets/ticket-1/github-issue"}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls=%v, want %v", calls, want)
	}
}

func TestMemberFeedbackSDKPreservesPublicErrors(t *testing.T) {
	for _, test := range []struct {
		name, code string
		status     int
		call       func(*Client) error
	}{
		{"invitation-target-denied", "ErrPermissionDenied", 403, func(c *Client) error {
			_, err := c.WorkspaceMembers("workspace-1").AcceptInvitation(t.Context(), "inv-1")
			return err
		}},
		{"admin-ticket-missing", "ErrFeedbackTicketNotFound", 404, func(c *Client) error {
			_, err := c.Agents("workspace-1").AdminFeedbackTicket(t.Context(), "ticket-1")
			return err
		}},
		{"reward-subject-missing", "ErrFeedbackRewardSubjectMissing", 400, func(c *Client) error {
			_, err := c.Agents("workspace-1").ApproveFeedbackReward(t.Context(), "ticket-1", &FeedbackTicketAdminUpdateRequest{})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(headerRequestID, "public-request-id")
				w.WriteHeader(test.status)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": test.code, "msg": "原始本地化错误"})
			}))
			t.Cleanup(server.Close)
			client, err := New(server.URL+"/newmoi", WithAPIKey("key"))
			if err != nil {
				t.Fatal(err)
			}
			err = test.call(client)
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code || failure.Status != test.status || failure.Message != "原始本地化错误" || failure.RequestID != "public-request-id" {
				t.Fatalf("public error=%#v", err)
			}
		})
	}
}
