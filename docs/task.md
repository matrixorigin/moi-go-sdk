# Task（任务管理）

任务管理接口用于查询和管理数据导入任务。本文档基于 `RawClient`，涵盖 `task.go` 中的方法。

## 功能总览

| 方法 | 说明 |
| ---- | ---- |
| `GetTask` | 根据任务 ID 获取任务的详细信息 |

> 所有示例默认已通过 `sdk.NewRawClient(baseURL, apiKey)` 创建 `rawClient`，并准备好 `ctx := context.Background()`。

## 获取任务信息

`GetTask` 用于获取指定任务的详细信息，包括任务状态、配置、结果等。

### 方法签名

```go
func (c *RawClient) GetTask(ctx context.Context, req *TaskInfoRequest, opts ...CallOption) (*TaskInfoResponse, error)
```

### 参数说明

**TaskInfoRequest 结构**:

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| TaskID | TaskID | 是 | 任务 ID |

### 返回值

**TaskInfoResponse 结构**:

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 任务 ID |
| SourceConnectorId | uint64 | 源连接器 ID |
| SourceConnectorType | string | 源连接器类型 |
| VolumeID | string | 目标卷 ID |
| VolumeName | string | 目标卷名称 |
| VolumePath | *FullPath | 目标卷完整路径 |
| Name | string | 任务名称 |
| Creator | string | 创建者 |
| Status | string | 任务状态 |
| SourceConfig | map[string]interface{} | 源配置 |
| StartAt | string | 开始时间 |
| EndAt | string | 结束时间 |
| CreatedAt | string | 创建时间 |
| UpdatedAt | string | 更新时间 |
| ConnectorName | string | 连接器名称 |
| TablePath | *FullPath | 表完整路径（如果导入到表） |
| SourceFiles | [][]string | 源文件列表 |
| LoadResults | []*LoadResult | 加载结果列表 |

**LoadResult 结构**:
- `Lines`: 加载的行数
- `Reason`: 失败原因（如果有）

### 示例

#### 基本使用

```go
resp, err := rawClient.GetTask(ctx, &sdk.TaskInfoRequest{
    TaskID: 123456,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Task ID: %s\n", resp.ID)
fmt.Printf("Task Name: %s\n", resp.Name)
fmt.Printf("Status: %s\n", resp.Status)
fmt.Printf("Volume ID: %s\n", resp.VolumeID)
fmt.Printf("Created At: %s\n", resp.CreatedAt)
```

#### 完整示例：上传文件并查询任务状态

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    
    "github.com/matrixorigin/moi-go-sdk/sdk"
)

func main() {
    // 创建客户端
    rawClient, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
    if err != nil {
        log.Fatal(err)
    }
    
    sdkClient := sdk.NewSDKClient(rawClient)
    ctx := context.Background()
    
    // 1. 创建测试卷（需要先有 catalog 和 database）
    // catalogID, _ := createCatalog(...)
    // databaseID, _ := createDatabase(...)
    // volumeID, _ := createVolume(...)
    volumeID := sdk.VolumeID("123456")
    
    // 2. 上传文件到卷
    testFilePath := "/path/to/test_file.txt"
    meta := sdk.FileMeta{
        Filename: "test_file.txt",
        Path:     "test_file.txt",
    }
    dedup := &sdk.DedupConfig{
        By:       []string{"name", "md5"},
        Strategy: "skip",
    }
    
    uploadResp, err := sdkClient.ImportLocalFileToVolume(ctx, testFilePath, volumeID, meta, dedup)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Upload successful, task_id: %d\n", uploadResp.TaskId)
    
    // 3. 查询任务信息
    taskResp, err := rawClient.GetTask(ctx, &sdk.TaskInfoRequest{
        TaskID: sdk.TaskID(uploadResp.TaskId),
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. 显示任务信息
    fmt.Printf("\nTask Information:\n")
    fmt.Printf("  ID: %s\n", taskResp.ID)
    fmt.Printf("  Name: %s\n", taskResp.Name)
    fmt.Printf("  Status: %s\n", taskResp.Status)
    fmt.Printf("  Volume ID: %s\n", taskResp.VolumeID)
    fmt.Printf("  Volume Name: %s\n", taskResp.VolumeName)
    fmt.Printf("  Creator: %s\n", taskResp.Creator)
    fmt.Printf("  Created At: %s\n", taskResp.CreatedAt)
    fmt.Printf("  Updated At: %s\n", taskResp.UpdatedAt)
    
    if taskResp.StartAt != "" {
        fmt.Printf("  Start At: %s\n", taskResp.StartAt)
    }
    if taskResp.EndAt != "" {
        fmt.Printf("  End At: %s\n", taskResp.EndAt)
    }
    
    // 5. 显示加载结果（如果有）
    if len(taskResp.LoadResults) > 0 {
        fmt.Printf("\nLoad Results:\n")
        for i, result := range taskResp.LoadResults {
            fmt.Printf("  File %d: %d lines", i+1, result.Lines)
            if result.Reason != "" {
                fmt.Printf(" (Reason: %s)", result.Reason)
            }
            fmt.Println()
        }
    }
    
    // 6. 显示源文件列表（如果有）
    if len(taskResp.SourceFiles) > 0 {
        fmt.Printf("\nSource Files:\n")
        for _, file := range taskResp.SourceFiles {
            if len(file) >= 2 {
                fmt.Printf("  %s: %s\n", file[0], file[1])
            }
        }
    }
}
```

#### 批量上传并查询任务

```go
ctx := context.Background()

// 批量上传多个文件
filePaths := []string{
    "/path/to/file1.docx",
    "/path/to/file2.docx",
    "/path/to/file3.docx",
}

metas := []sdk.FileMeta{
    {Filename: "file1.docx", Path: "file1.docx"},
    {Filename: "file2.docx", Path: "file2.docx"},
    {Filename: "file3.docx", Path: "file3.docx"},
}

uploadResp, err := sdkClient.ImportLocalFilesToVolume(ctx, filePaths, volumeID, metas, dedup)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Uploaded %d files, task_id: %d\n", len(filePaths), uploadResp.TaskId)

// 查询任务状态
taskResp, err := rawClient.GetTask(ctx, &sdk.TaskInfoRequest{
    TaskID: sdk.TaskID(uploadResp.TaskId),
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Task Status: %s\n", taskResp.Status)
```

## 任务状态说明

任务状态通常包括以下值：

- `pending`: 等待中
- `running`: 运行中
- `success`: 成功
- `failed`: 失败
- `paused`: 已暂停

具体状态值可能因服务版本而异，请参考实际返回的状态值。

## 注意事项

1. **任务 ID**: 任务 ID 通常来自上传操作的响应（`UploadFileResponse.TaskId`）
2. **异步操作**: 文件上传和导入是异步操作，可能需要一些时间完成
3. **状态查询**: 可以使用 `GetTask` 定期查询任务状态，直到任务完成
4. **错误处理**: 如果任务失败，`LoadResults` 中的 `Reason` 字段会包含失败原因
5. **权限要求**: 查询任务信息需要相应的权限（`PrivID_GetLoadTask`）

## 相关接口

- [ImportLocalFileToVolume](../sdk-client.md#importlocalfiletovolume) - 上传单个文件到卷
- [ImportLocalFilesToVolume](../sdk-client.md#importlocalfilestovolume) - 批量上传文件到卷
- [ImportLocalFileToTable](../sdk-client.md#importlocalfiletotable) - 导入文件到表

