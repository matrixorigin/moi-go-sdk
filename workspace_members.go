package sdk

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	product "github.com/matrixorigin/moi-go-sdk/v5/product"
)

var mainlandChinaInvitationMobileNumberRegexp = regexp.MustCompile(`^1\d{10}$`)
var githubLoginRegexp = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)

type AuditLogListResult = product.AuditLogListResult
type AuditLogOverviewResult = product.AuditLogOverviewResult
type CurrentWorkspacePrincipalResult = product.CurrentWorkspacePrincipalResult
type WorkspaceMemberListResult = product.WorkspaceMemberListResult
type WorkspaceMember = product.WorkspaceMember
type WorkspaceMemberInviteResult = product.WorkspaceMemberInviteResult
type WorkspaceInvitationAcceptResult = product.WorkspaceInvitationAcceptResult

// AcceptInvitation accepts a pending invitation as the authenticated target user.
func (s *WorkspaceMemberService) AcceptInvitation(ctx context.Context, invitationID string) (*WorkspaceInvitationAcceptResult, error) {
	invitationPath, err := pathID(invitationID)
	if err != nil {
		return nil, err
	}
	out := &product.WorkspaceInvitationAcceptResult{}
	err = s.do(ctx, http.MethodPost, "/workspaces/invitations/"+invitationPath+"/accept", map[string]any{}, out)
	return out, err
}

func (s *WorkspaceMemberService) CurrentPrincipal(ctx context.Context) (*CurrentWorkspacePrincipalResult, error) {
	out := &product.CurrentWorkspacePrincipalResult{}
	err := s.do(ctx, http.MethodPost, "/user/info", nil, out)
	return out, err
}

func (s *WorkspaceMemberService) Members(ctx context.Context, opts ...WorkspaceMemberListOption) (*WorkspaceMemberListResult, error) {
	body := memberListBody(opts)
	out := &product.WorkspaceMemberListResult{}
	err := s.do(ctx, http.MethodPost, "/user/list", body, out)
	return out, err
}

func (s *WorkspaceMemberService) Invite(ctx context.Context, name string, defaultRoleID int, opts ...CreateUserOption) (*WorkspaceMemberInviteResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	body := map[string]any{"name": name}
	options := &createUserOptions{}
	for _, opt := range opts {
		opt(options)
	}
	hasEmail := options.email != nil && strings.TrimSpace(*options.email) != ""
	hasMobile := options.mobile != nil && strings.TrimSpace(*options.mobile) != ""
	hasGitHub := options.githubLogin != nil && strings.TrimSpace(*options.githubLogin) != ""
	contacts := 0
	for _, present := range []bool{hasEmail, hasMobile, hasGitHub} {
		if present {
			contacts++
		}
	}
	if contacts == 0 {
		return nil, ErrInvitationContactRequired
	}
	if contacts > 1 {
		return nil, ErrInvitationContactConflict
	}
	if hasMobile && !mainlandChinaInvitationMobileNumberRegexp.MatchString(*options.mobile) {
		return nil, ErrInvitationMobileNumberInvalid
	}
	if hasGitHub {
		login := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(*options.githubLogin), "@")))
		if !githubLoginRegexp.MatchString(login) {
			return nil, ErrInvitationGitHubLoginInvalid
		}
		*options.githubLogin = login
	}
	if err := validateAssignedDefaultRole(options.roleIDs, defaultRoleID); err != nil {
		return nil, err
	}
	body["default_role_id"] = defaultRoleID
	if options.email != nil {
		body["email"] = *options.email
	}
	if options.mobile != nil {
		body["mobile_number"] = *options.mobile
	}
	if options.githubLogin != nil {
		body["github_login"] = *options.githubLogin
	}
	if options.description != nil {
		body["description"] = *options.description
	}
	if options.roleIDs != nil {
		body["role_id_list"] = options.roleIDs
	}
	if options.tags != nil {
		body["tag_list"] = options.tags
	}
	if options.autoAccept {
		body["auto_accept"] = true
	}
	out := &product.WorkspaceMemberInviteResult{}
	err := s.do(ctx, http.MethodPost, "/user/create", body, out)
	return out, err
}

func (s *WorkspaceMemberService) Member(ctx context.Context, userID string) (*WorkspaceMember, error) {
	if userID == "" {
		return nil, ErrIDRequired
	}
	out := &product.WorkspaceMember{}
	err := s.do(ctx, http.MethodPost, "/user/detail_info", map[string]any{"id": userID}, out)
	return out, err
}

func (s *WorkspaceMemberService) Update(ctx context.Context, userID, name, description string, opts ...UpdateUserOption) error {
	if userID == "" {
		return ErrIDRequired
	}
	if name == "" {
		return ErrNameRequired
	}
	options := &updateUserOptions{}
	for _, opt := range opts {
		opt(options)
	}
	tags := options.tags
	if tags == nil {
		tags = []userTag{}
	}
	return s.postNoData(ctx, "/user/update_info", map[string]any{"id": userID, "name": name, "description": description, "tag_list": tags})
}

func validateAssignedDefaultRole(roleIDs []int, defaultRoleID int) error {
	if defaultRoleID <= 0 {
		return ErrDefaultRoleRequired
	}
	for _, roleID := range roleIDs {
		if roleID == defaultRoleID {
			return nil
		}
	}
	return ErrDefaultRoleNotAssigned
}

func (s *WorkspaceMemberService) Delete(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrIDRequired
	}
	return s.postNoData(ctx, "/user/delete", map[string]any{"id": userID})
}

func (s *WorkspaceMemberService) UpdateStatus(ctx context.Context, userID, action string) error {
	if userID == "" {
		return ErrIDRequired
	}
	if action == "" {
		return ErrActionRequired
	}
	return s.postNoData(ctx, "/user/update_status", map[string]any{"id": userID, "action": action})
}

// CreateUserTag creates a user subject-attribute tag definition.
func (s *WorkspaceMemberService) CreateUserTag(ctx context.Context, name string) (*IDResult, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	out := &product.IDResult{}
	err := s.do(ctx, http.MethodPost, "/user/tag/create", map[string]any{"name": name}, out)
	return out, err
}

// UserTags lists user subject-attribute tag definitions.
func (s *WorkspaceMemberService) UserTags(ctx context.Context) (*DataTagListResult, error) {
	var out DataTagListResult
	err := s.do(ctx, http.MethodPost, "/user/tag/list", map[string]any{}, &out)
	return &out, err
}

// UpdateUserTag updates a user subject-attribute tag definition.
func (s *WorkspaceMemberService) UpdateUserTag(ctx context.Context, tagID int, name string) (*IDResult, error) {
	if tagID == 0 {
		return nil, ErrIDRequired
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	out := &product.IDResult{}
	err := s.do(ctx, http.MethodPost, "/user/tag/update", map[string]any{"id": tagID, "name": name}, out)
	return out, err
}

// DeleteUserTag deletes a user subject-attribute tag definition.
func (s *WorkspaceMemberService) DeleteUserTag(ctx context.Context, tagID int) (*IDResult, error) {
	if tagID == 0 {
		return nil, ErrIDRequired
	}
	out := &product.IDResult{}
	err := s.do(ctx, http.MethodPost, "/user/tag/delete", map[string]any{"id": tagID}, out)
	return out, err
}

func (s *WorkspaceMemberService) AuditLogs(ctx context.Context, opts ...AuditLogOption) (*AuditLogListResult, error) {
	out := &product.AuditLogListResult{}
	err := s.do(ctx, http.MethodGet, withQuery("/audit/logs", opts), nil, out)
	return out, err
}

func (s *WorkspaceMemberService) AuditLogOverview(ctx context.Context) (*AuditLogOverviewResult, error) {
	out := &product.AuditLogOverviewResult{}
	err := s.do(ctx, http.MethodGet, "/audit/logs/overview", nil, out)
	return out, err
}

func (s *WorkspaceMemberService) post(ctx context.Context, path string, body any) (*ProductData, error) {
	var out ProductData
	err := s.do(ctx, http.MethodPost, path, body, &out)
	return &out, err
}

func (s *WorkspaceMemberService) postNoData(ctx context.Context, path string, body any) error {
	return s.do(ctx, http.MethodPost, path, body, nil)
}

func memberListBody(opts []WorkspaceMemberListOption) map[string]any {
	options := &workspaceMemberListOptions{}
	for _, opt := range opts {
		opt(options)
	}
	body := map[string]any{}
	if options.page != nil {
		body["page"] = *options.page
	}
	if options.pageSize != nil {
		body["page_size"] = *options.pageSize
	}
	if options.filters != nil {
		body["filters"] = options.filters
	}
	return body
}
