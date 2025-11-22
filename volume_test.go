package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVolumeLiveFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	catalogID, markCatalogDeleted := createTestCatalog(t, client)
	databaseID, markDatabaseDeleted := createTestDatabase(t, client, catalogID)

	volumeName := randomName("sdk-volume-")
	createResp, err := client.CreateVolume(ctx, &VolumeCreateRequest{
		Name:       volumeName,
		DatabaseID: databaseID,
		Comment:    "sdk volume",
	})
	require.NoError(t, err)
	volumeID := createResp.VolumeID

	volumeDeleted := false
	t.Cleanup(func() {
		if volumeDeleted {
			return
		}
		if _, err := client.DeleteVolume(ctx, &VolumeDeleteRequest{VolumeID: volumeID}); err != nil {
			t.Logf("cleanup delete volume failed: %v", err)
		}
	})

	infoResp, err := client.GetVolume(ctx, &VolumeInfoRequest{VolumeID: volumeID})
	require.NoError(t, err)
	require.Equal(t, volumeName, infoResp.VolumeName)

	_, err = client.UpdateVolume(ctx, &VolumeUpdateRequest{
		VolumeID: volumeID,
		Name:     randomName("sdk-volume-updated-"),
		Comment:  "updated",
	})
	require.NoError(t, err)

	refResp, err := client.GetVolumeRefList(ctx, &VolumeRefListRequest{VolumeID: volumeID})
	require.NoError(t, err)
	require.NotNil(t, refResp)

	fullPathResp, err := client.GetVolumeFullPath(ctx, &VolumeFullPathRequest{VolumeIDList: []VolumeID{volumeID}})
	require.NoError(t, err)
	require.NotNil(t, fullPathResp)

	_, err = client.AddVolumeWorkflowRef(ctx, &VolumeAddRefWorkflowRequest{VolumeID: volumeID})
	require.NoError(t, err)

	_, err = client.RemoveVolumeWorkflowRef(ctx, &VolumeRemoveRefWorkflowRequest{VolumeID: volumeID})
	require.NoError(t, err)

	_, err = client.DeleteVolume(ctx, &VolumeDeleteRequest{VolumeID: volumeID})
	require.NoError(t, err)
	volumeDeleted = true

	_, err = client.DeleteDatabase(ctx, &DatabaseDeleteRequest{DatabaseID: databaseID})
	require.NoError(t, err)
	markDatabaseDeleted()

	_, err = client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID})
	require.NoError(t, err)
	markCatalogDeleted()
}

func TestVolumeNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"Create", func() error { _, err := client.CreateVolume(ctx, nil); return err }},
		{"Delete", func() error { _, err := client.DeleteVolume(ctx, nil); return err }},
		{"Update", func() error { _, err := client.UpdateVolume(ctx, nil); return err }},
		{"Info", func() error { _, err := client.GetVolume(ctx, nil); return err }},
		{"RefList", func() error { _, err := client.GetVolumeRefList(ctx, nil); return err }},
		{"FullPath", func() error { _, err := client.GetVolumeFullPath(ctx, nil); return err }},
		{"AddRefWorkflow", func() error { _, err := client.AddVolumeWorkflowRef(ctx, nil); return err }},
		{"RemoveRefWorkflow", func() error { _, err := client.RemoveVolumeWorkflowRef(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}
