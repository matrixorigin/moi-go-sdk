package sdk

import (
	"net/http"
	"strconv"
	"strings"
)

const headerMOIRoleID = "X-MOI-Role-ID"

type workspaceCurrentRole struct {
	workspaceID string
	roleID      string
}

// ValidateCurrentRoleID validates the canonical external Role Candidate format.
// Binding and lifecycle validation remain authoritative in Core.
func ValidateCurrentRoleID(roleID string) error {
	parsed, err := strconv.ParseInt(roleID, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != roleID {
		return ErrCurrentRoleInvalid
	}
	return nil
}

func resolveWorkspaceCurrentRoles(selections []workspaceCurrentRole) (map[string]string, error) {
	roles := make(map[string]string, len(selections))
	for _, selection := range selections {
		if selection.workspaceID == "" {
			return nil, ErrWorkspaceRequired
		}
		if err := ValidateCurrentRoleID(selection.roleID); err != nil {
			return nil, err
		}
		if existing, ok := roles[selection.workspaceID]; ok && existing != selection.roleID {
			return nil, ErrWorkspaceCurrentRoleConflict
		}
		roles[selection.workspaceID] = selection.roleID
	}
	return roles, nil
}

// applyWorkspaceScope finalizes SDK-owned workspace and Role headers after
// generic custom headers and request modifiers have run.
func (c *Client) applyWorkspaceScope(header http.Header, workspaceID string) {
	deleteHeaderFold(header, headerWorkspaceID)
	deleteHeaderFold(header, headerMOIRoleID)
	if workspaceID == "" {
		return
	}
	header.Set(headerWorkspaceID, workspaceID)
	if roleID, ok := c.workspaceCurrentRoleIDs[workspaceID]; ok {
		header.Set(headerMOIRoleID, roleID)
	}
}

func deleteHeaderFold(header http.Header, target string) {
	for key := range header {
		if strings.EqualFold(key, target) {
			delete(header, key)
		}
	}
}
