package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := newTestClient(t)

	// Test nil request
	resp, err := client.GetTask(ctx, nil)
	require.Nil(t, resp)
	require.ErrorIs(t, err, ErrNilRequest)

	// Test empty task ID
	resp, err = client.GetTask(ctx, &TaskInfoRequest{TaskID: 0})
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "task_id is required")
}

func TestImportLocalFileToVolumeAndGetTask(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	rawClient := newTestClient(t)
	sdkClient := NewSDKClient(rawClient)

	// Create test catalog, database, and volume
	catalogID, _ := createTestCatalog(t, rawClient)
	databaseID, _ := createTestDatabase(t, rawClient, catalogID)
	volumeID, _ := createTestVolume(t, rawClient, databaseID)

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFileName := "test_file.txt"
	testFilePath := filepath.Join(tmpDir, testFileName)
	testContent := "This is a test file for ImportLocalFileToVolume"
	err := os.WriteFile(testFilePath, []byte(testContent), 0644)
	require.NoError(t, err)

	// Upload file to volume using ImportLocalFileToVolume
	meta := FileMeta{
		Filename: testFileName,
		Path:     testFileName,
	}
	dedup := &DedupConfig{
		By:       []string{"name", "md5"},
		Strategy: "skip",
	}

	uploadResp, err := sdkClient.ImportLocalFileToVolume(ctx, testFilePath, volumeID, meta, dedup)
	require.NoError(t, err)
	require.NotNil(t, uploadResp)
	require.NotZero(t, uploadResp.TaskId, "TaskId should be returned from upload")

	t.Logf("Upload successful, task_id: %d", uploadResp.TaskId)

	// Get task information using GetTask
	taskResp, err := rawClient.GetTask(ctx, &TaskInfoRequest{
		TaskID: TaskID(uploadResp.TaskId),
	})
	require.NoError(t, err)
	require.NotNil(t, taskResp)

	// Verify task information
	require.Equal(t, fmt.Sprintf("%d", uploadResp.TaskId), taskResp.ID, "Task ID should match")
	require.NotEmpty(t, taskResp.Status, "Task status should be present")
	require.NotEmpty(t, taskResp.CreatedAt, "CreatedAt should be present")
	require.Equal(t, string(volumeID), taskResp.VolumeID, "Volume ID should match")

	t.Logf("Task retrieved successfully:")
	t.Logf("  ID: %s", taskResp.ID)
	t.Logf("  Name: %s", taskResp.Name)
	t.Logf("  Status: %s", taskResp.Status)
	t.Logf("  VolumeID: %s", taskResp.VolumeID)
	t.Logf("  CreatedAt: %s", taskResp.CreatedAt)
}

func TestImportLocalFilesToVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	rawClient := newTestClient(t)
	sdkClient := NewSDKClient(rawClient)

	// Create test catalog, database, and volume
	catalogID, _ := createTestCatalog(t, rawClient)
	databaseID, _ := createTestDatabase(t, rawClient, catalogID)
	volumeID, _ := createTestVolume(t, rawClient, databaseID)

	// Create temporary test files
	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "test_file1.txt")
	testFile2 := filepath.Join(tmpDir, "test_file2.txt")
	testContent1 := "This is test file 1 for ImportLocalFilesToVolume"
	testContent2 := "This is test file 2 for ImportLocalFilesToVolume"
	err := os.WriteFile(testFile1, []byte(testContent1), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte(testContent2), 0644)
	require.NoError(t, err)

	// Test with provided metas
	metas := []FileMeta{
		{Filename: "test_file1.txt", Path: "test_file1.txt"},
		{Filename: "test_file2.txt", Path: "test_file2.txt"},
	}
	dedup := &DedupConfig{
		By:       []string{"name", "md5"},
		Strategy: "skip",
	}

	uploadResp, err := sdkClient.ImportLocalFilesToVolume(ctx, []string{testFile1, testFile2}, volumeID, metas, dedup)
	require.NoError(t, err)
	require.NotNil(t, uploadResp)
	require.NotZero(t, uploadResp.TaskId, "TaskId should be returned from upload")

	t.Logf("Upload successful, task_id: %d", uploadResp.TaskId)

	// Test with auto-generated metas (empty metas array)
	uploadResp2, err := sdkClient.ImportLocalFilesToVolume(ctx, []string{testFile1, testFile2}, volumeID, nil, dedup)
	require.NoError(t, err)
	require.NotNil(t, uploadResp2)
	require.NotZero(t, uploadResp2.TaskId)

	t.Logf("Upload with auto-generated metas successful, task_id: %d", uploadResp2.TaskId)
}

func TestImportLocalFilesToVolumeErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sdkClient := NewSDKClient(&RawClient{baseURL: "http://example.com", apiKey: "test-key"})

	// Test empty file paths
	resp, err := sdkClient.ImportLocalFilesToVolume(ctx, []string{}, "123456", nil, nil)
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one file path is required")

	// Test empty volume ID
	resp, err = sdkClient.ImportLocalFilesToVolume(ctx, []string{"/path/to/file.txt"}, "", nil, nil)
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "volume_id is required")

	// Test mismatched metas length
	resp, err = sdkClient.ImportLocalFilesToVolume(ctx, []string{"/path/to/file1.txt", "/path/to/file2.txt"}, "123456", []FileMeta{{Filename: "file1.txt", Path: "file1.txt"}}, nil)
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "metas array length")

	// Test empty file path in array
	resp, err = sdkClient.ImportLocalFilesToVolume(ctx, []string{"", "/path/to/file2.txt"}, "123456", nil, nil)
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "file_path[0] is empty")
}
