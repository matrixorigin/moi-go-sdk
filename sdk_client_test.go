package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTableRole_EmptyRoleName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rawClient := &RawClient{}
	client := NewSDKClient(rawClient)

	roleID, created, err := client.CreateTableRole(ctx, "", "test comment", []TablePrivInfo{})
	require.Equal(t, RoleID(0), roleID)
	require.False(t, created)
	require.Error(t, err)
	require.Contains(t, err.Error(), "role name is required")
}

func TestCreateTableRole_LiveFlow(t *testing.T) {
	ctx := context.Background()
	rawClient, err := NewRawClient(testBaseURL, testAPIKey)
	require.NoError(t, err)
	client := NewSDKClient(rawClient)

	// Create a test role with table privileges
	roleName := randomName("sdk_table_role_")
	comment := "SDK test table role"
	tablePrivs := []TablePrivInfo{
		{
			TableID:   TableID(1),
			PrivCodes: []PrivCode{PrivCode_TableSelect, PrivCode_TableInsert},
		},
		{
			TableID:   TableID(2),
			PrivCodes: []PrivCode{PrivCode_TableSelect, PrivCode_TableUpdate, PrivCode_TableDelete},
		},
	}

	// First call: should create the role
	roleID1, created1, err := client.CreateTableRole(ctx, roleName, comment, tablePrivs)
	require.NoError(t, err)
	require.NotEqual(t, RoleID(0), roleID1)
	require.True(t, created1, "first call should create the role")
	t.Logf("Created role with ID: %d", roleID1)

	// Cleanup: delete the role after test
	defer func() {
		if _, err := rawClient.DeleteRole(ctx, &RoleDeleteRequest{RoleID: roleID1}); err != nil {
			t.Logf("cleanup delete role failed: %v", err)
		}
	}()

	// Second call: should return existing role
	roleID2, created2, err := client.CreateTableRole(ctx, roleName, comment, tablePrivs)
	require.NoError(t, err)
	require.Equal(t, roleID1, roleID2, "should return the same role ID")
	require.False(t, created2, "second call should not create a new role")
	t.Logf("Existing role returned with ID: %d", roleID2)

	// Test with different role name (should create new role)
	roleName2 := randomName("sdk_table_role_")
	comment2 := "SDK test table role 2"
	tablePrivs2 := []TablePrivInfo{
		{
			TableID:   TableID(3),
			PrivCodes: []PrivCode{PrivCode_ShowTables},
		},
	}

	roleID3, created3, err := client.CreateTableRole(ctx, roleName2, comment2, tablePrivs2)
	require.NoError(t, err)
	require.NotEqual(t, roleID1, roleID3, "should create a different role")
	require.True(t, created3, "should create a new role")
	t.Logf("Created second role with ID: %d", roleID3)

	// Cleanup second role
	defer func() {
		if _, err := rawClient.DeleteRole(ctx, &RoleDeleteRequest{RoleID: roleID3}); err != nil {
			t.Logf("cleanup delete second role failed: %v", err)
		}
	}()
}

func TestNewSDKClient_NilRawClient(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when RawClient is nil")
		}
	}()

	NewSDKClient(nil)
	t.Error("should have panicked")
}

func TestTablePrivInfo_Structure(t *testing.T) {
	t.Parallel()

	// Test that TablePrivInfo can be constructed properly
	tablePriv := TablePrivInfo{
		TableID:   TableID(123),
		PrivCodes: []PrivCode{PrivCode_TableSelect, PrivCode_TableInsert},
	}

	require.Equal(t, TableID(123), tablePriv.TableID)
	require.Len(t, tablePriv.PrivCodes, 2)
	require.Equal(t, PrivCode_TableSelect, tablePriv.PrivCodes[0])
	require.Equal(t, PrivCode_TableInsert, tablePriv.PrivCodes[1])
}
