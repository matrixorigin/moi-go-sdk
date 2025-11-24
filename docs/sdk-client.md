# SDK Client（高级客户端）

`SDKClient` 提供高级封装接口，简化常见业务场景的操作。它基于 `RawClient` 构建，封装了多个 API 调用以实现更复杂的业务逻辑。

## 创建 SDKClient

```go
// 先创建 RawClient
rawClient, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
if err != nil {
    log.Fatal(err)
}

// 创建 SDKClient
sdkClient := sdk.NewSDKClient(rawClient)
```

## 接口列表

- [CreateTableRole](#createtablerole) - 创建表角色（自动检查是否存在）
- [UpdateTableRole](#updatetablerole) - 更新表角色权限
- [ImportLocalFileToTable](#importlocalfiletotable) - 导入本地文件到表

## CreateTableRole

创建表角色，如果角色已存在则返回现有角色 ID。

### 方法签名

```go
func (c *SDKClient) CreateTableRole(ctx context.Context, roleName string, comment string, tablePrivs []TablePrivInfo) (roleID RoleID, created bool, err error)
```

### 参数说明

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| roleName | string | 是 | 角色名称 |
| comment | string | 否 | 角色描述/备注 |
| tablePrivs | []TablePrivInfo | 是 | 表权限信息列表 |

**TablePrivInfo 结构**:
- `TableID`: 表 ID
- `AuthorityCodeList`: 权限代码列表（推荐使用，支持规则）
- `PrivCodes`: 简单权限代码列表（已废弃，向后兼容）

**AuthorityCodeAndRule 结构**:
- `Code`: 权限代码（如 "DT8" 表示 SELECT）
- `RuleList`: 规则列表（可选，用于行/列级权限）

### 返回值

- `roleID`: 角色 ID（新建或已存在的）
- `created`: 是否为新创建的角色（true 表示新建，false 表示已存在）
- `error`: 错误信息

### 示例

#### 基本使用

```go
ctx := context.Background()

roleID, created, err := sdkClient.CreateTableRole(ctx, "my-role", "Role description", []sdk.TablePrivInfo{
    {
        TableID: 123,
        AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
            {
                Code:     "DT8", // SELECT 权限
                RuleList: nil,
            },
            {
                Code:     "DT9", // INSERT 权限
                RuleList: nil,
            },
        },
    },
})
if err != nil {
    log.Fatal(err)
}

if created {
    fmt.Printf("Created new role: %d\n", roleID)
} else {
    fmt.Printf("Role already exists: %d\n", roleID)
}
```

#### 使用规则（行/列级权限）

```go
ctx := context.Background()

roleID, created, err := sdkClient.CreateTableRole(ctx, "restricted-role", "Role with rules", []sdk.TablePrivInfo{
    {
        TableID: 123,
        AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
            {
                Code: "DT8", // SELECT 权限
                RuleList: []*sdk.TableRowColRule{
                    {
                        Column:   "department",
                        Relation: "and",
                        ExpressionList: []*sdk.TableRowColExpression{
                            {
                                Operator:   "=",
                                Expression: "IT",
                            },
                        },
                    },
                },
            },
        },
    },
})
```

#### 向后兼容（使用 PrivCodes）

```go
ctx := context.Background()

roleID, created, err := sdkClient.CreateTableRole(ctx, "simple-role", "Simple role", []sdk.TablePrivInfo{
    {
        TableID: 123,
        PrivCodes: []sdk.PrivCode{
            sdk.PrivCode_TableSelect, // DT8
            sdk.PrivCode_TableInsert, // DT9
        },
    },
})
```

## UpdateTableRole

更新表角色的权限信息。

### 方法签名

```go
func (c *SDKClient) UpdateTableRole(ctx context.Context, roleID RoleID, comment string, tablePrivs []TablePrivInfo, globalPrivs []string) error
```

### 参数说明

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| roleID | RoleID | 是 | 要更新的角色 ID |
| comment | string | 否 | 新的角色描述（空字符串表示保持现有） |
| tablePrivs | []TablePrivInfo | 是 | 新的表权限信息列表 |
| globalPrivs | []string | 否 | 全局权限代码列表（nil 表示保持现有，空切片表示删除所有） |

### 返回值

- `error`: 错误信息

### 示例

#### 更新表权限

```go
ctx := context.Background()

err := sdkClient.UpdateTableRole(ctx, 456, "", []sdk.TablePrivInfo{
    {
        TableID: 123,
        AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
            {
                Code:     "DT8",
                RuleList: nil,
            },
        },
    },
    {
        TableID: 124,
        AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
            {
                Code:     "DT9",
                RuleList: nil,
            },
        },
    },
}, nil) // nil 表示保持现有全局权限
if err != nil {
    log.Fatal(err)
}
```

#### 更新角色描述和全局权限

```go
ctx := context.Background()

err := sdkClient.UpdateTableRole(ctx, 456, "Updated description", []sdk.TablePrivInfo{
    // ... 表权限
}, []string{"U1", "R1"}) // 设置全局权限
if err != nil {
    log.Fatal(err)
}
```

#### 删除所有全局权限

```go
ctx := context.Background()

err := sdkClient.UpdateTableRole(ctx, 456, "", []sdk.TablePrivInfo{
    // ... 表权限
}, []string{}) // 空切片表示删除所有全局权限
if err != nil {
    log.Fatal(err)
}
```

## ImportLocalFileToTable

导入已上传的本地文件到表。这是一个高级接口，简化了文件导入流程。

### 方法签名

```go
func (c *SDKClient) ImportLocalFileToTable(ctx context.Context, tableConfig *TableConfig) (*UploadFileResponse, error)
```

### 参数说明

**TableConfig 结构**:

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| ConnFileIDs | []string | 是 | 已上传文件的 ID 列表（来自 UploadLocalFile） |
| NewTable | bool | 是 | true 表示创建新表，false 表示导入到现有表 |
| DatabaseID | DatabaseID | 是 | 数据库 ID |
| TableID | TableID | 条件 | 当 NewTable=false 时必需，目标表 ID |
| ExistedTable | []FileAndTableColumnMapping | 条件 | 当 NewTable=false 时可选，文件列到表列的映射 |
| CreateTable | *CreateTableConfig | 条件 | 当 NewTable=true 时可选，表创建配置 |
| IsColumnName | bool | 否 | 是否包含列名行 |
| ColumnNameRow | int | 否 | 列名行号 |
| RowStart | int | 否 | 数据起始行 |
| Conflict | ConflictPolicy | 否 | 冲突处理策略（0:失败, 1:跳过, 2:替换） |

**FileAndTableColumnMapping 结构**:
- `TableColumn`: 表列名
- `Column`: 文件列名或默认值或 NULL
- `ColNumInFile`: 文件中的列号（从1开始）

### 返回值

- `*UploadFileResponse`: 上传响应，包含任务 ID
- `error`: 错误信息

### 示例

#### 导入到新表

```go
ctx := context.Background()

// 1. 先上传文件
file, err := os.Open("data.csv")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

uploadResp, err := rawClient.UploadLocalFile(ctx, &sdk.LocalFileUploadRequest{
    Files: []sdk.FileUploadItem{
        {
            File:     file,
            FileName: "data.csv",
        },
    },
    Meta: []sdk.FileMeta{
        {
            Filename: "data.csv",
            Path:     "/",
        },
    },
})
if err != nil {
    log.Fatal(err)
}

connFileID := uploadResp.ConnFileIds[0]

// 2. 预览文件结构
previewResp, err := rawClient.FilePreview(ctx, &sdk.FilePreviewRequest{
    ConnFileId: connFileID,
})
if err != nil {
    log.Fatal(err)
}

// 3. 构建表配置（基于预览结果）
tableConfig := &sdk.TableConfig{
    NewTable:    true,
    DatabaseID:  123,
    ConnFileIDs: []string{connFileID},
    CreateTable: &sdk.CreateTableConfig{
        Name:        "my_table",
        Description: "My table",
        TableColumn: []sdk.TableColumn{
            {
                Number:         1,
                ColumnName:     "id",
                DataType:       "int",
                IsKey:          true,
                ColNumInFile:   1,
            },
            {
                Number:         2,
                ColumnName:     "name",
                DataType:       "varchar",
                ColNumInFile:   2,
            },
        },
    },
    IsColumnName:  true,
    ColumnNameRow: 1,
    RowStart:      2,
    Conflict:      sdk.ConflictPolicySkip,
}

// 4. 导入到表
importResp, err := sdkClient.ImportLocalFileToTable(ctx, tableConfig)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Import task ID: %d\n", importResp.TaskId)
```

#### 导入到现有表

```go
ctx := context.Background()

tableConfig := &sdk.TableConfig{
    NewTable:    false,
    DatabaseID:  123,
    TableID:     456, // 现有表 ID
    ConnFileIDs: []string{connFileID},
    ExistedTable: []sdk.FileAndTableColumnMapping{
        {
            TableColumn:  "id",
            Column:       "id",
            ColNumInFile: 1,
        },
        {
            TableColumn:  "name",
            Column:       "name",
            ColNumInFile: 2,
        },
    },
    Conflict: sdk.ConflictPolicyReplace,
}

importResp, err := sdkClient.ImportLocalFileToTable(ctx, tableConfig)
if err != nil {
    log.Fatal(err)
}
```

## 完整示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
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
    
    // 1. 创建表角色
    roleID, created, err := sdkClient.CreateTableRole(ctx, "data-reader", "Read-only role", []sdk.TablePrivInfo{
        {
            TableID: 123,
            AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
                {
                    Code:     "DT8", // SELECT
                    RuleList: nil,
                },
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Role %d (created: %v)\n", roleID, created)
    
    // 2. 更新角色权限
    err = sdkClient.UpdateTableRole(ctx, roleID, "Updated description", []sdk.TablePrivInfo{
        {
            TableID: 123,
            AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
                {
                    Code:     "DT8",
                    RuleList: nil,
                },
                {
                    Code:     "DT9",
                    RuleList: nil,
                },
            },
        },
    }, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. 导入文件到表
    file, _ := os.Open("data.csv")
    defer file.Close()
    
    uploadResp, _ := rawClient.UploadLocalFile(ctx, &sdk.LocalFileUploadRequest{
        Files: []sdk.FileUploadItem{{File: file, FileName: "data.csv"}},
        Meta:  []sdk.FileMeta{{Filename: "data.csv", Path: "/"}},
    })
    
    tableConfig := &sdk.TableConfig{
        NewTable:    true,
        DatabaseID:  123,
        ConnFileIDs: uploadResp.ConnFileIds,
        // ... 其他配置
    }
    
    importResp, err := sdkClient.ImportLocalFileToTable(ctx, tableConfig)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Import task: %d\n", importResp.TaskId)
}
```

## 注意事项

1. **CreateTableRole**: 会自动检查角色是否存在，避免重复创建
2. **UpdateTableRole**: 
   - `comment` 为空字符串时保持现有描述
   - `globalPrivs` 为 `nil` 时保持现有全局权限
   - `globalPrivs` 为空切片时删除所有全局权限
3. **ImportLocalFileToTable**: 
   - 使用固定的 VolumeID ("123456")
   - 文件必须已通过 `UploadLocalFile` 上传
   - 需要先调用 `FilePreview` 获取文件结构信息

