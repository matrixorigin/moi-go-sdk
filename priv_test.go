package sdk

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivLiveFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	catalogID, markCatalogDeleted := createTestCatalog(t, client)

	// Create a role with permissions
	roleID, markRoleDeleted := createTestRole(t, client, []string{string(PrivCode_QueryCatalog), string(PrivCode_UpdateCatalog)})

	// Create a user with the role
	createResp, err := client.CreateUser(ctx, &UserCreateRequest{
		UserName:    strings.ToLower(randomUserName()),
		Password:    "TestPwd123!",
		RoleIDList:  []RoleID{roleID},
		Description: "sdk test user for priv check",
		Phone:       "12345678901",
		Email:       "sdk-priv@example.com",
	})
	require.NoError(t, err)
	userID := createResp.UserID

	userDeleted := false
	t.Cleanup(func() {
		if userDeleted {
			return
		}
		if _, err := client.DeleteUser(ctx, &UserDeleteUserRequest{UserID: userID}); err != nil {
			t.Logf("cleanup delete user failed: %v", err)
		}
		if _, err := client.DeleteRole(ctx, &RoleDeleteRequest{RoleID: roleID}); err != nil {
			t.Logf("cleanup delete role failed: %v", err)
		}
		markRoleDeleted()
	})

	// List objects by category with uid header
	// listReq := &PrivListObjByCategoryRequest{ObjType: ObjTypeCatalog.String()}
	// listResp, err := client.ListObjectsByCategory(ctx, listReq)
	// require.NoError(t, err)
	// require.NotNil(t, listResp)

	// Cleanup
	_, err = client.DeleteUser(ctx, &UserDeleteUserRequest{UserID: userID})
	require.NoError(t, err)
	userDeleted = true

	_, err = client.DeleteRole(ctx, &RoleDeleteRequest{RoleID: roleID})
	require.NoError(t, err)
	markRoleDeleted()

	_, err = client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID})
	require.NoError(t, err)
	markCatalogDeleted()
}

func TestPrivNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"ListByCategory", func() error { _, err := client.ListObjectsByCategory(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}
