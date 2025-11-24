# 客户端初始化

本文档介绍如何创建和配置 MOI Go SDK 客户端。

## 创建 RawClient

### 基本创建

```go
client, err := sdk.NewRawClient(
    "https://api.example.com",  // baseURL: 服务端地址
    "your-api-key",             // apiKey: API 密钥
)
if err != nil {
    log.Fatal(err)
}
```

### 参数说明

- **baseURL** (string, 必需): 服务端的基础 URL，必须包含协议（http:// 或 https://）和主机名
- **apiKey** (string, 必需): API 密钥，用于身份验证

### 配置选项

#### WithHTTPTimeout

设置 HTTP 请求超时时间：

```go
client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithHTTPTimeout(60 * time.Second),
)
```

**默认值**: 30 秒

#### WithHTTPClient

使用自定义的 HTTP 客户端：

```go
customClient := &http.Client{
    Timeout: 120 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns: 100,
    },
}

client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithHTTPClient(customClient),
)
```

#### WithUserAgent

设置自定义 User-Agent 请求头：

```go
client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithUserAgent("my-app/1.0.0"),
)
```

**默认值**: `matrixflow-sdk-go/0.1.0`

#### WithDefaultHeader

添加默认请求头（所有请求都会包含）：

```go
client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithDefaultHeader("X-Custom-Header", "value"),
)
```

#### WithDefaultHeaders

批量添加默认请求头：

```go
headers := http.Header{}
headers.Set("X-Custom-1", "value1")
headers.Set("X-Custom-2", "value2")

client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithDefaultHeaders(headers),
)
```

### 组合使用多个选项

```go
client, err := sdk.NewRawClient(
    "https://api.example.com",
    "your-api-key",
    sdk.WithHTTPTimeout(60 * time.Second),
    sdk.WithUserAgent("my-app/1.0.0"),
    sdk.WithDefaultHeader("X-Request-Source", "sdk"),
)
```

## 创建 SDKClient

`SDKClient` 基于 `RawClient` 创建：

```go
// 先创建 RawClient
rawClient, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
if err != nil {
    log.Fatal(err)
}

// 创建 SDKClient
sdkClient := sdk.NewSDKClient(rawClient)
```

**注意**: `NewSDKClient` 如果传入 `nil` 会 panic，请确保 `rawClient` 不为空。

## 请求选项 (CallOption)

在调用 API 方法时，可以使用 `CallOption` 自定义单个请求的行为：

### WithRequestID

设置请求 ID（会作为 `X-Request-ID` 请求头发送）：

```go
resp, err := client.CreateCatalog(ctx, req, 
    sdk.WithRequestID("my-request-id-123"),
)
```

### WithHeader

为单个请求添加或覆盖请求头：

```go
resp, err := client.CreateCatalog(ctx, req,
    sdk.WithHeader("X-Custom-Header", "value"),
)
```

### WithHeaders

为单个请求批量添加请求头：

```go
headers := http.Header{}
headers.Set("X-Custom-1", "value1")
headers.Set("X-Custom-2", "value2")

resp, err := client.CreateCatalog(ctx, req,
    sdk.WithHeaders(headers),
)
```

### WithQueryParam

添加查询参数：

```go
resp, err := client.ListCatalogs(ctx,
    sdk.WithQueryParam("page", "1"),
    sdk.WithQueryParam("size", "10"),
)
```

### WithQuery

批量添加查询参数：

```go
query := url.Values{}
query.Set("page", "1")
query.Set("size", "10")
query.Add("filter", "value1")
query.Add("filter", "value2")

resp, err := client.ListCatalogs(ctx,
    sdk.WithQuery(query),
)
```

### 组合使用多个请求选项

```go
resp, err := client.CreateCatalog(ctx, req,
    sdk.WithRequestID("req-123"),
    sdk.WithHeader("X-Custom", "value"),
    sdk.WithQueryParam("debug", "true"),
)
```

## 完整示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/matrixorigin/moi-go-sdk/sdk"
)

func main() {
    // 创建客户端，配置超时和自定义请求头
    client, err := sdk.NewRawClient(
        "https://api.example.com",
        "your-api-key",
        sdk.WithHTTPTimeout(60 * time.Second),
        sdk.WithUserAgent("my-app/1.0.0"),
        sdk.WithDefaultHeader("X-Request-Source", "go-sdk"),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 创建高级客户端
    sdkClient := sdk.NewSDKClient(client)
    
    ctx := context.Background()
    
    // 使用请求选项
    resp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
        CatalogName: "my-catalog",
        Comment:     "My catalog",
    }, sdk.WithRequestID("catalog-create-001"))
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Created catalog: %d\n", resp.CatalogID)
}
```

## 注意事项

1. **baseURL 格式**: 必须包含协议（http:// 或 https://），URL 末尾的斜杠会被自动移除
2. **API Key**: 必须非空，前后空格会被自动去除
3. **超时设置**: 建议根据实际网络情况设置合理的超时时间
4. **请求 ID**: 建议为每个请求设置唯一的请求 ID，便于问题追踪
5. **线程安全**: `RawClient` 和 `SDKClient` 都是线程安全的，可以在多个 goroutine 中并发使用

