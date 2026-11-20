package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type workspaceMemberContractFilter struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type workspaceMemberContractTag struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

type workspaceMemberContractInput struct {
	InvitationID       string                          `json:"invitation_id"`
	UserID             string                          `json:"user_id"`
	Name               string                          `json:"name"`
	Description        string                          `json:"description"`
	DescriptionPresent bool                            `json:"description_present"`
	Action             string                          `json:"action"`
	Email              string                          `json:"email"`
	EmailPresent       bool                            `json:"email_present"`
	DefaultRoleID      int                             `json:"default_role_id"`
	RoleIDs            []int                           `json:"role_ids"`
	AutoAccept         bool                            `json:"auto_accept"`
	TagID              int                             `json:"tag_id"`
	Tags               []workspaceMemberContractTag    `json:"tags"`
	Page               int                             `json:"page"`
	PagePresent        bool                            `json:"page_present"`
	PageSize           int                             `json:"page_size"`
	PageSizePresent    bool                            `json:"page_size_present"`
	Filters            []workspaceMemberContractFilter `json:"filters"`
}

type workspaceMemberHTTPContract struct {
	Name      string                       `json:"name"`
	Operation string                       `json:"operation"`
	Method    string                       `json:"method"`
	Path      string                       `json:"path"`
	Query     map[string]string            `json:"query"`
	Input     workspaceMemberContractInput `json:"input"`
	Request   json.RawMessage              `json:"request"`
	Response  json.RawMessage              `json:"response"`
	Status    int                          `json:"status"`
}

func workspaceMemberContractJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&value)
	return value
}

func workspaceMemberAuditOptions(values map[string]string) ([]AuditLogOption, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	options := make([]AuditLogOption, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		switch key {
		case "method":
			options = append(options, WithAuditMethod(value))
		case "result":
			options = append(options, WithAuditResult(value))
		case "status_code", "page", "page_size":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			switch key {
			case "status_code":
				options = append(options, WithAuditStatusCode(parsed))
			case "page":
				options = append(options, WithPage(parsed))
			case "page_size":
				options = append(options, WithPageSize(parsed))
			}
		case "start_time", "end_time":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, err
			}
			if key == "start_time" {
				options = append(options, WithAuditStartTime(parsed))
			} else {
				options = append(options, WithAuditEndTime(parsed))
			}
		default:
			return nil, fmt.Errorf("unknown audit query parameter %q", key)
		}
	}
	return options, nil
}

func TestWorkspaceMemberSharedHTTPContracts(t *testing.T) {
	data, err := os.ReadFile("testdata/workspace_member_http_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []workspaceMemberHTTPContract
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			var received any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedQuery := url.Values{}
				for key, value := range scenario.Query {
					expectedQuery.Set(key, value)
				}
				if r.Method != scenario.Method || r.URL.EscapedPath() != scenario.Path || !reflect.DeepEqual(r.URL.Query(), expectedQuery) || r.Header.Get("X-Workspace-ID") != "workspace-1" || r.Header.Get("X-API-Key") != "user-key" {
					t.Errorf("request=%s %s query=%v headers=%v", r.Method, r.URL.RequestURI(), r.URL.Query(), r.Header)
				}
				raw, _ := io.ReadAll(r.Body)
				if len(raw) > 0 {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					_ = decoder.Decode(&received)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(scenario.Status)
				_, _ = w.Write(scenario.Response)
			}))
			defer server.Close()
			client, err := New(server.URL+"/newmoi", WithAPIKey("user-key"))
			if err != nil {
				t.Fatal(err)
			}
			service := client.WorkspaceMembers("workspace-1")
			input := scenario.Input
			var result proto.Message
			switch scenario.Operation {
			case "accept-invitation":
				result, err = service.AcceptInvitation(t.Context(), input.InvitationID)
			case "current-principal":
				result, err = service.CurrentPrincipal(t.Context())
			case "members":
				var options []WorkspaceMemberListOption
				if input.PagePresent {
					options = append(options, WithMemberPage(input.Page))
				}
				if input.PageSizePresent {
					options = append(options, WithMemberPageSize(input.PageSize))
				}
				for _, filter := range input.Filters {
					options = append(options, WithMemberFilter(filter.Name, filter.Values...))
				}
				result, err = service.Members(t.Context(), options...)
			case "invite":
				var options []CreateUserOption
				if input.EmailPresent {
					options = append(options, WithUserEmail(input.Email))
				}
				if input.DescriptionPresent {
					options = append(options, WithUserDescription(input.Description))
				}
				if input.RoleIDs != nil {
					options = append(options, WithUserRoleIDs(input.RoleIDs...))
				}
				if input.AutoAccept {
					options = append(options, WithUserAutoAccept(true))
				}
				for _, tag := range input.Tags {
					options = append(options, WithCreateUserTag(tag.ID, tag.Value))
				}
				result, err = service.Invite(t.Context(), input.Name, input.DefaultRoleID, options...)
			case "member":
				result, err = service.Member(t.Context(), input.UserID)
			case "update":
				options := make([]UpdateUserOption, 0, len(input.Tags))
				for _, tag := range input.Tags {
					options = append(options, WithUserTag(tag.ID, tag.Value))
				}
				err = service.Update(t.Context(), input.UserID, input.Name, input.Description, options...)
			case "delete":
				err = service.Delete(t.Context(), input.UserID)
			case "update-status":
				err = service.UpdateStatus(t.Context(), input.UserID, input.Action)
			case "create-user-tag":
				result, err = service.CreateUserTag(t.Context(), input.Name)
			case "user-tags":
				result, err = service.UserTags(t.Context())
			case "update-user-tag":
				result, err = service.UpdateUserTag(t.Context(), input.TagID, input.Name)
			case "delete-user-tag":
				result, err = service.DeleteUserTag(t.Context(), input.TagID)
			case "audit-logs":
				var options []AuditLogOption
				options, err = workspaceMemberAuditOptions(scenario.Query)
				if err == nil {
					result, err = service.AuditLogs(t.Context(), options...)
				}
			case "audit-log-overview":
				result, err = service.AuditLogOverview(t.Context())
			default:
				t.Fatalf("unknown operation %s", scenario.Operation)
			}
			if err != nil {
				t.Fatal(err)
			}
			if result != nil {
				var envelope struct {
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(scenario.Response, &envelope); err != nil {
					t.Fatal(err)
				}
				expected := result.ProtoReflect().New().Interface()
				if err := protojson.Unmarshal(envelope.Data, expected); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(result, expected) {
					t.Fatalf("result=%v want=%v", result, expected)
				}
			}
			if !reflect.DeepEqual(received, workspaceMemberContractJSON(scenario.Request)) {
				t.Fatalf("body=%#v want=%#v", received, workspaceMemberContractJSON(scenario.Request))
			}
		})
	}
}
