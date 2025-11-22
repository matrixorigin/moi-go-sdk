package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogLiveCRUD(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	createReq := &CatalogCreateRequest{
		CatalogName: randomName("sdk-catalog-"),
	}
	createResp, err := client.CreateCatalog(ctx, createReq)
	require.NoError(t, err)
	require.NotZero(t, createResp.CatalogID)

	catalogID := createResp.CatalogID
	cleanupDone := false
	t.Cleanup(func() {
		if cleanupDone {
			return
		}
		if _, err := client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID}); err != nil {
			t.Logf("cleanup delete catalog failed: %v", err)
		}
	})

	infoResp, err := client.GetCatalog(ctx, &CatalogInfoRequest{CatalogID: catalogID})
	require.NoError(t, err)
	require.Equal(t, createReq.CatalogName, infoResp.CatalogName)

	updatedName := randomName("sdk-catalog-updated-")
	_, err = client.UpdateCatalog(ctx, &CatalogUpdateRequest{
		CatalogID:   catalogID,
		CatalogName: updatedName,
	})
	require.NoError(t, err)

	infoResp, err = client.GetCatalog(ctx, &CatalogInfoRequest{CatalogID: catalogID})
	require.NoError(t, err)
	require.Equal(t, updatedName, infoResp.CatalogName)

	listResp, err := client.ListCatalogs(ctx)
	require.NoError(t, err)
	require.NotNil(t, listResp)

	treeResp, err := client.GetCatalogTree(ctx)
	require.NoError(t, err)
	require.NotNil(t, treeResp)

	refResp, err := client.GetCatalogRefList(ctx, &CatalogRefListRequest{CatalogID: catalogID})
	require.NoError(t, err)
	require.NotNil(t, refResp)

	_, err = client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID})
	require.NoError(t, err)
	cleanupDone = true
}

func TestCatalogNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"Create", func() error { _, err := client.CreateCatalog(ctx, nil); return err }},
		{"Delete", func() error { _, err := client.DeleteCatalog(ctx, nil); return err }},
		{"Update", func() error { _, err := client.UpdateCatalog(ctx, nil); return err }},
		{"Get", func() error { _, err := client.GetCatalog(ctx, nil); return err }},
		{"RefList", func() error { _, err := client.GetCatalogRefList(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}
