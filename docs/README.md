# MOI Go SDK 文档

MOI Go SDK 是一个用于与 MOI Catalog Service 交互的 Go 语言客户端库。它提供了两种客户端：

- **RawClient**: 底层客户端，提供对 API 的直接访问
- **SDKClient**: 高级客户端，提供更便捷的业务逻辑封装

## 目录

- [快速开始](./getting-started.md) - 安装和初始化客户端
- [客户端初始化](./client-initialization.md) - 详细配置选项
- [Catalog（目录）](./catalog.md) - 目录管理接口
- [Database（数据库）](./database.md) - 数据库管理接口
- [Table（表）](./table.md) - 表管理接口
- [Volume（卷）](./volume.md) - 数据卷管理接口
- [File（文件）](./file.md) - 文件管理接口
- [Folder（文件夹）](./folder.md) - 文件夹管理接口
- [Connector（连接器）](./connector.md) - 文件上传和数据导入接口
- [User（用户）](./user.md) - 用户管理接口
- [Role（角色）](./role.md) - 角色管理接口
- [Privilege（权限）](./privilege.md) - 权限查询接口
- [SDK Client（高级客户端）](./sdk-client.md) - 高级封装接口
- [错误处理](./error-handling.md) - 错误类型和处理方式

## 架构概览

```
┌─────────────────┐
│   SDKClient     │  ← 高级客户端（业务逻辑封装）
│  (High-level)   │
└────────┬────────┘
         │
         │ wraps
         ▼
┌─────────────────┐
│   RawClient     │  ← 底层客户端（直接 API 访问）
│   (Low-level)   │
└────────┬────────┘
         │
         │ HTTP
         ▼
┌─────────────────┐
│ Catalog Service │
└─────────────────┘
```

## 核心概念

### RawClient

`RawClient` 提供对 Catalog Service API 的直接访问。每个方法对应一个 API 端点，参数和返回值与 API 定义一一对应。

**特点：**
- 直接映射 API 端点
- 参数和返回值与 API 定义一致
- 适合需要精确控制 API 调用的场景

### SDKClient

`SDKClient` 在 `RawClient` 基础上提供高级封装，简化常见业务场景的操作。

**特点：**
- 封装多个 API 调用
- 提供便捷的业务逻辑
- 自动处理复杂场景（如角色创建时的存在性检查）

## 使用建议

1. **新用户**：建议从 `SDKClient` 开始，它提供了更友好的接口
2. **需要精确控制**：使用 `RawClient` 直接访问 API
3. **混合使用**：可以同时使用两种客户端，`SDKClient` 内部使用 `RawClient`

## 示例

### 使用 RawClient

```go
import "github.com/matrixorigin/moi-go-sdk/sdk"

// 创建客户端
client, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
if err != nil {
    log.Fatal(err)
}

// 创建目录
ctx := context.Background()
resp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
    CatalogName: "my-catalog",
    Comment:     "My catalog description",
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Created catalog with ID: %d\n", resp.CatalogID)
```

### 使用 SDKClient

```go
import "github.com/matrixorigin/moi-go-sdk/sdk"

// 创建底层客户端
rawClient, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
if err != nil {
    log.Fatal(err)
}

// 创建高级客户端
sdkClient := sdk.NewSDKClient(rawClient)

// 导入本地文件到表
ctx := context.Background()
tableConfig := &sdk.TableConfig{
    NewTable: true,
    DatabaseID: 123,
    ConnFileIDs: []string{"file-id-123"},
    // ... 其他配置
}
resp, err := sdkClient.ImportLocalFileToTable(ctx, tableConfig)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Import task ID: %d\n", resp.TaskId)
```

## 更多信息

详细的接口文档请参考各个模块的文档页面。

