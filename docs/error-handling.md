# 错误处理

本文档介绍 MOI Go SDK 中的错误类型和处理方式。

## 错误类型

SDK 定义了三种主要的错误类型：

### 1. ErrNilRequest

当请求参数为 `nil` 时返回。

```go
var ErrNilRequest = errors.New("sdk: request payload cannot be nil")
```

**示例**:
```go
resp, err := client.CreateCatalog(ctx, nil)
if err != nil {
    if err == sdk.ErrNilRequest {
        fmt.Println("Request cannot be nil")
    }
}
```

### 2. APIError

API 业务逻辑错误，由服务端返回。

```go
type APIError struct {
    Code       string  // 错误代码
    Message    string  // 错误消息
    RequestID  string  // 请求 ID
    HTTPStatus int     // HTTP 状态码
}
```

**常见错误代码**:
- `ErrInternal`: 内部错误
- 其他业务错误代码（如目录不存在、名称冲突等）

**示例**:
```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        fmt.Printf("API Error: %s\n", apiErr.Message)
        fmt.Printf("Error Code: %s\n", apiErr.Code)
        fmt.Printf("Request ID: %s\n", apiErr.RequestID)
        fmt.Printf("HTTP Status: %d\n", apiErr.HTTPStatus)
        
        // 根据错误代码处理
        switch apiErr.Code {
        case "ErrInternal":
            // 处理内部错误
        default:
            // 处理其他错误
        }
    }
}
```

### 3. HTTPError

HTTP 网络错误，发生在无法解析响应信封之前。

```go
type HTTPError struct {
    StatusCode int     // HTTP 状态码
    Body       []byte  // 响应体
}
```

**示例**:
```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    if httpErr, ok := err.(*sdk.HTTPError); ok {
        fmt.Printf("HTTP Error: %d\n", httpErr.StatusCode)
        fmt.Printf("Response Body: %s\n", string(httpErr.Body))
        
        // 根据状态码处理
        switch httpErr.StatusCode {
        case 401:
            fmt.Println("Unauthorized - check your API key")
        case 404:
            fmt.Println("Not Found")
        case 500:
            fmt.Println("Server Error")
        default:
            fmt.Printf("Unexpected status: %d\n", httpErr.StatusCode)
        }
    }
}
```

## 错误处理最佳实践

### 1. 统一错误处理函数

```go
func handleError(err error) {
    if err == nil {
        return
    }
    
    // 检查是否为 API 错误
    if apiErr, ok := err.(*sdk.APIError); ok {
        log.Printf("API Error [%s]: %s (Request ID: %s)",
            apiErr.Code, apiErr.Message, apiErr.RequestID)
        return
    }
    
    // 检查是否为 HTTP 错误
    if httpErr, ok := err.(*sdk.HTTPError); ok {
        log.Printf("HTTP Error: %d - %s",
            httpErr.StatusCode, string(httpErr.Body))
        return
    }
    
    // 检查是否为 nil request 错误
    if err == sdk.ErrNilRequest {
        log.Println("Error: Request cannot be nil")
        return
    }
    
    // 其他错误
    log.Printf("Unexpected error: %v", err)
}
```

### 2. 错误重试

对于网络错误，可以实现重试逻辑：

```go
func createCatalogWithRetry(client *sdk.RawClient, req *sdk.CatalogCreateRequest, maxRetries int) (*sdk.CatalogCreateResponse, error) {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        resp, err := client.CreateCatalog(context.Background(), req)
        if err == nil {
            return resp, nil
        }
        
        lastErr = err
        
        // 只对 HTTP 错误进行重试
        if httpErr, ok := err.(*sdk.HTTPError); ok {
            if httpErr.StatusCode >= 500 {
                // 服务器错误，可以重试
                time.Sleep(time.Duration(i+1) * time.Second)
                continue
            }
        }
        
        // 其他错误不重试
        return nil, err
    }
    
    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
```

### 3. 错误包装

使用 `fmt.Errorf` 包装错误以添加上下文：

```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    return fmt.Errorf("failed to create catalog: %w", err)
}
```

### 4. 错误日志记录

记录详细的错误信息以便调试：

```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        log.WithFields(log.Fields{
            "error_code":  apiErr.Code,
            "error_msg":   apiErr.Message,
            "request_id":  apiErr.RequestID,
            "http_status": apiErr.HTTPStatus,
            "catalog_name": req.CatalogName,
        }).Error("Failed to create catalog")
    }
    return err
}
```

## 常见错误场景

### 1. 资源不存在

```go
resp, err := client.GetCatalog(ctx, &sdk.CatalogInfoRequest{
    CatalogID: 999,
})
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        if strings.Contains(apiErr.Message, "not exists") {
            fmt.Println("Catalog does not exist")
        }
    }
}
```

### 2. 名称冲突

```go
resp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
    CatalogName: "existing-catalog",
})
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        if strings.Contains(apiErr.Message, "exists") {
            fmt.Println("Catalog name already exists")
        }
    }
}
```

### 3. 权限不足

```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        if strings.Contains(apiErr.Message, "permission") || 
           strings.Contains(apiErr.Message, "privilege") {
            fmt.Println("Insufficient permissions")
        }
    }
}
```

### 4. 参数验证错误

```go
resp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
    CatalogName: "", // 空名称
})
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        if strings.Contains(apiErr.Message, "required") ||
           strings.Contains(apiErr.Message, "invalid") {
            fmt.Println("Invalid request parameters")
        }
    }
}
```

## 错误处理示例

完整的错误处理示例：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strings"
    
    "github.com/matrixorigin/moi-go-sdk/sdk"
)

func main() {
    client, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // 创建目录
    resp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
        CatalogName: "my-catalog",
        Comment:     "My catalog",
    })
    
    if err != nil {
        // 处理 API 错误
        if apiErr, ok := err.(*sdk.APIError); ok {
            switch {
            case strings.Contains(apiErr.Message, "exists"):
                log.Printf("Catalog already exists: %s", apiErr.Message)
            case strings.Contains(apiErr.Message, "permission"):
                log.Printf("Permission denied: %s", apiErr.Message)
            default:
                log.Printf("API Error [%s]: %s", apiErr.Code, apiErr.Message)
            }
            return
        }
        
        // 处理 HTTP 错误
        if httpErr, ok := err.(*sdk.HTTPError); ok {
            log.Printf("HTTP Error %d: %s", httpErr.StatusCode, string(httpErr.Body))
            return
        }
        
        // 处理其他错误
        log.Fatal(err)
    }
    
    fmt.Printf("Created catalog: %d\n", resp.CatalogID)
}
```

## 注意事项

1. **总是检查错误**: 不要忽略方法返回的错误
2. **使用类型断言**: 使用类型断言区分不同类型的错误
3. **记录请求 ID**: API 错误包含 RequestID，用于服务端问题追踪
4. **不要重试业务错误**: 只对网络错误进行重试，不要重试业务逻辑错误
5. **提供上下文**: 在包装错误时提供足够的上下文信息

