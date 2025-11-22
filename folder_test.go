package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFolderLiveFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	catalogID, markCatalogDeleted := createTestCatalog(t, client)
	databaseID, markDatabaseDeleted := createTestDatabase(t, client, catalogID)
	volumeID, markVolumeDeleted := createTestVolume(t, client, databaseID)

	createResp, err := client.CreateFolder(ctx, &FolderCreateRequest{
		Name:     randomName("sdk-folder-"),
		VolumeID: volumeID,
	})
	require.NoError(t, err)
	folderID := createResp.FolderID

	folderDeleted := false
	t.Cleanup(func() {
		if folderDeleted {
			return
		}
		if _, err := client.DeleteFolder(ctx, &FolderDeleteRequest{FolderID: folderID}); err != nil {
			t.Logf("cleanup delete folder failed: %v", err)
		}
	})

	_, err = client.UpdateFolder(ctx, &FolderUpdateRequest{
		FolderID: folderID,
		Name:     randomName("sdk-folder-updated-"),
	})
	require.NoError(t, err)

	_, err = client.GetFolderRefList(ctx, &FolderRefListRequest{FolderID: folderID})
	require.NoError(t, err)

	_, err = client.CleanFolder(ctx, &FolderCleanRequest{FolderID: folderID})
	require.NoError(t, err)

	_, err = client.DeleteFolder(ctx, &FolderDeleteRequest{FolderID: folderID})
	require.NoError(t, err)
	folderDeleted = true

	_, err = client.DeleteVolume(ctx, &VolumeDeleteRequest{VolumeID: volumeID})
	require.NoError(t, err)
	markVolumeDeleted()

	_, err = client.DeleteDatabase(ctx, &DatabaseDeleteRequest{DatabaseID: databaseID})
	require.NoError(t, err)
	markDatabaseDeleted()

	_, err = client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID})
	require.NoError(t, err)
	markCatalogDeleted()
}

func TestFolderNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"Create", func() error { _, err := client.CreateFolder(ctx, nil); return err }},
		{"Update", func() error { _, err := client.UpdateFolder(ctx, nil); return err }},
		{"Delete", func() error { _, err := client.DeleteFolder(ctx, nil); return err }},
		{"Clean", func() error { _, err := client.CleanFolder(ctx, nil); return err }},
		{"RefList", func() error { _, err := client.GetFolderRefList(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}
