package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileLiveFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	catalogID, markCatalogDeleted := createTestCatalog(t, client)
	databaseID, markDatabaseDeleted := createTestDatabase(t, client, catalogID)
	volumeID, markVolumeDeleted := createTestVolume(t, client, databaseID)

	folderResp, err := client.CreateFolder(ctx, &FolderCreateRequest{
		Name:     randomName("sdk-folder-"),
		VolumeID: volumeID,
	})
	require.NoError(t, err)
	folderID := folderResp.FolderID
	folderDeleted := false
	t.Cleanup(func() {
		if folderDeleted {
			return
		}
		if _, err := client.DeleteFolder(ctx, &FolderDeleteRequest{FolderID: folderID}); err != nil {
			t.Logf("cleanup delete folder failed: %v", err)
		}
	})

	createResp, err := client.CreateFile(ctx, &FileCreateRequest{
		Name:     randomName("sdk-file-"),
		VolumeID: volumeID,
		ParentID: folderID,
		ShowType: "normal",
		Size:     1,
		SavePath: "/tmp",
	})
	require.NoError(t, err)
	fileID := createResp.FileID

	_, err = client.UpdateFile(ctx, &FileUpdateRequest{
		FileID: fileID,
		Name:   randomName("sdk-file-updated-"),
	})
	require.NoError(t, err)

	infoResp, err := client.GetFile(ctx, &FileInfoRequest{FileID: fileID})
	require.NoError(t, err)
	require.Equal(t, fileID, infoResp.ID)

	listReq := &FileListRequest{}
	listReq.Filters = append(listReq.Filters, CommonFilter{
		Name:   "volume_id",
		Values: []string{string(volumeID)},
	})
	listResp, err := client.ListFiles(ctx, listReq)
	require.NoError(t, err)
	require.NotNil(t, listResp)

	_, err = client.DeleteFile(ctx, &FileDeleteRequest{FileID: fileID})
	require.NoError(t, err)

	folderDeleted = true
	_, err = client.DeleteFolder(ctx, &FolderDeleteRequest{FolderID: folderID})
	require.NoError(t, err)

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

func TestFileNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"Create", func() error { _, err := client.CreateFile(ctx, nil); return err }},
		{"Update", func() error { _, err := client.UpdateFile(ctx, nil); return err }},
		{"Delete", func() error { _, err := client.DeleteFile(ctx, nil); return err }},
		{"DeleteRef", func() error { _, err := client.DeleteFileRef(ctx, nil); return err }},
		{"Info", func() error { _, err := client.GetFile(ctx, nil); return err }},
		{"List", func() error { _, err := client.ListFiles(ctx, nil); return err }},
		{"Upload", func() error { _, err := client.UploadFile(ctx, nil); return err }},
		{"Download", func() error { _, err := client.GetFileDownloadLink(ctx, nil); return err }},
		{"PreviewLink", func() error { _, err := client.GetFilePreviewLink(ctx, nil); return err }},
		{"PreviewStream", func() error { _, err := client.GetFilePreviewStream(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}
