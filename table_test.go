package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTableLiveFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	catalogID, markCatalogDeleted := createTestCatalog(t, client)
	databaseID, markDatabaseDeleted := createTestDatabase(t, client, catalogID)

	tableName := randomName("sdk-table-")
	columns := []Column{
		{Name: "id", Type: "int", IsPk: true},
		{Name: "name", Type: "varchar(255)"},
	}
	createResp, err := client.CreateTable(ctx, &TableCreateRequest{
		DatabaseID: databaseID,
		Name:       tableName,
		Columns:    columns,
		Comment:    "sdk test table",
	})
	require.NoError(t, err)
	tableID := createResp.TableID

	tableDeleted := false
	t.Cleanup(func() {
		if tableDeleted {
			return
		}
		if _, err := client.DeleteTable(ctx, &TableDeleteRequest{TableID: tableID}); err != nil {
			t.Logf("cleanup delete table failed: %v", err)
		}
	})

	infoResp, err := client.GetTable(ctx, &TableInfoRequest{TableID: tableID})
	require.NoError(t, err)
	require.Equal(t, tableName, infoResp.Name)

	exists, err := client.CheckTableExists(ctx, &TableExistRequest{
		DatabaseID: databaseID,
		Name:       tableName,
	})
	require.NoError(t, err)
	require.True(t, exists)

	previewResp, err := client.PreviewTable(ctx, &TablePreviewRequest{TableID: tableID, Lines: 5})
	require.NoError(t, err)
	require.NotNil(t, previewResp)

	truncResp, err := client.TruncateTable(ctx, &TableTruncateRequest{TableID: tableID})
	require.NoError(t, err)
	require.NotNil(t, truncResp)

	fullPathResp, err := client.GetTableFullPath(ctx, &TableFullPathRequest{TableIDList: []TableID{tableID}})
	require.NoError(t, err)
	require.NotNil(t, fullPathResp)

	refListResp, err := client.GetTableRefList(ctx, &TableRefListRequest{TableID: tableID})
	require.NoError(t, err)
	require.NotNil(t, refListResp)

	_, err = client.DeleteTable(ctx, &TableDeleteRequest{TableID: tableID})
	require.NoError(t, err)
	tableDeleted = true

	exists, err = client.CheckTableExists(ctx, &TableExistRequest{
		DatabaseID: databaseID,
		Name:       tableName,
	})
	require.NoError(t, err)
	require.False(t, exists)

	_, err = client.DeleteDatabase(ctx, &DatabaseDeleteRequest{DatabaseID: databaseID})
	require.NoError(t, err)
	markDatabaseDeleted()

	_, err = client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID})
	require.NoError(t, err)
	markCatalogDeleted()
}

func TestTableNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"Create", func() error { _, err := client.CreateTable(ctx, nil); return err }},
		{"Info", func() error { _, err := client.GetTable(ctx, nil); return err }},
		{"Exist", func() error { _, err := client.CheckTableExists(ctx, nil); return err }},
		{"Preview", func() error { _, err := client.PreviewTable(ctx, nil); return err }},
		{"Load", func() error { _, err := client.LoadTable(ctx, nil); return err }},
		{"Download", func() error { _, err := client.GetTableDownloadLink(ctx, nil); return err }},
		{"Truncate", func() error { _, err := client.TruncateTable(ctx, nil); return err }},
		{"Delete", func() error { _, err := client.DeleteTable(ctx, nil); return err }},
		{"FullPath", func() error { _, err := client.GetTableFullPath(ctx, nil); return err }},
		{"RefList", func() error { _, err := client.GetTableRefList(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}
