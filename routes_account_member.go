package sdk

// AccountAndMemberRouteMappings declares account and canonical workspace-member coverage.
func AccountAndMemberRouteMappings() []RouteMapping {
	return []RouteMapping{
		{Language: "go", SDKMethod: "Client.Account().CurrentCredit", Method: "GET", Path: "/newmoi/billing/credits/current", Kind: "read"},
		{Language: "go", SDKMethod: "Client.Account().DBConnection", Method: "GET", Path: "/newmoi/user/me/db-connection", Kind: "read"},

		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).CurrentPrincipal", Method: "POST", Path: "/newmoi/user/info", Kind: "read"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).Members", Method: "POST", Path: "/newmoi/user/list", Kind: "read"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).Invite", Method: "POST", Path: "/newmoi/user/create", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).AcceptInvitation", Method: "POST", Path: "/newmoi/workspaces/invitations/:invitation_id/accept", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).Member", Method: "POST", Path: "/newmoi/user/detail_info", Kind: "read"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).Update", Method: "POST", Path: "/newmoi/user/update_info", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).Delete", Method: "POST", Path: "/newmoi/user/delete", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).UpdateStatus", Method: "POST", Path: "/newmoi/user/update_status", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).UserTags", Method: "POST", Path: "/newmoi/user/tag/list", Kind: "read"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).CreateUserTag", Method: "POST", Path: "/newmoi/user/tag/create", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).UpdateUserTag", Method: "POST", Path: "/newmoi/user/tag/update", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).DeleteUserTag", Method: "POST", Path: "/newmoi/user/tag/delete", Kind: "write"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).AuditLogs", Method: "GET", Path: "/newmoi/audit/logs", Kind: "read"},
		{Language: "go", SDKMethod: "Client.WorkspaceMembers(workspaceID).AuditLogOverview", Method: "GET", Path: "/newmoi/audit/logs/overview", Kind: "read"},
	}
}
