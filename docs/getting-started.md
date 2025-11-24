# 快速开始

本指南将帮助您快速开始使用 MOI Go SDK。

## 安装

```bash
go get github.com/matrixorigin/moi-go-sdk
```

## 基本使用

### 1. 创建客户端

首先，您需要创建一个 `RawClient` 实例：

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/matrixorigin/moi-go-sdk/sdk"
)

func main() {
    // 创建客户端
    client, err := sdk.NewRawClient(
        "https://api.example.com",  // 服务端地址
        "your-api-key",              // API Key
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 使用客户端...
}
```

### 2. 配置选项

您可以使用选项函数自定义客户端行为：

```go
client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithHTTPTimeout(60 * time.Second),  // 设置超时时间
    sdk.WithUserAgent("my-app/1.0"),        // 自定义 User-Agent
    sdk.WithDefaultHeader("X-Custom", "value"), // 添加默认请求头
)
```

### 3. 执行 API 调用

```go
ctx := context.Background()

// 创建目录
createResp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
    CatalogName: "my-catalog",
    Comment:     "My first catalog",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created catalog ID: %d\n", createResp.CatalogID)

// 获取目录信息
infoResp, err := client.GetCatalog(ctx, &sdk.CatalogInfoRequest{
    CatalogID: createResp.CatalogID,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Catalog name: %s\n", infoResp.CatalogName)
```

### 4. 错误处理

SDK 定义了两种错误类型：

```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    // API 错误（业务逻辑错误）
    if apiErr, ok := err.(*sdk.APIError); ok {
        fmt.Printf("API Error: %s (code: %s)\n", apiErr.Message, apiErr.Code)
        return
    }
    
    // HTTP 错误（网络或服务器错误）
    if httpErr, ok := err.(*sdk.HTTPError); ok {
        fmt.Printf("HTTP Error: %d\n", httpErr.StatusCode)
        return
    }
    
    // 其他错误
    log.Fatal(err)
}
```

### 5. 使用高级客户端

```go
// 创建底层客户端
rawClient, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
if err != nil {
    log.Fatal(err)
}

// 创建高级客户端
sdkClient := sdk.NewSDKClient(rawClient)

// 使用高级接口
ctx := context.Background()
roleID, created, err := sdkClient.CreateTableRole(ctx, "my-role", "Role description", []sdk.TablePrivInfo{
    {
        TableID: 123,
        AuthorityCodeList: []*sdk.AuthorityCodeAndRule{
            {
                Code: "DT8", // SELECT 权限
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

## 下一步

- 查看 [客户端初始化文档](./client-initialization.md) 了解详细配置选项
- 查看各个模块的文档了解具体接口使用方法
- 查看 [错误处理文档](./error-handling.md) 了解错误处理最佳实践

