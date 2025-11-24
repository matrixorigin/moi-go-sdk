# Catalog（目录）接口

Catalog 是数据组织的顶层结构，用于管理数据库、表和卷。

## 接口列表

- [CreateCatalog](#createcatalog) - 创建目录
- [DeleteCatalog](#deletecatalog) - 删除目录
- [UpdateCatalog](#updatecatalog) - 更新目录
- [GetCatalog](#getcatalog) - 获取目录信息
- [ListCatalogs](#listcatalogs) - 列出所有目录
- [GetCatalogTree](#getcatalogtree) - 获取目录树
- [GetCatalogRefList](#getcatalogreflist) - 获取目录引用列表

## CreateCatalog

创建新的目录。

### 方法签名

```go
func (c *RawClient) CreateCatalog(ctx context.Context, req *CatalogCreateRequest, opts ...CallOption) (*CatalogCreateResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| CatalogName | string | 是 | 目录名称 |
| Comment | string | 否 | 目录描述/备注 |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| CatalogID | CatalogID | 创建的目录 ID |

### 示例

```go
ctx := context.Background()

// 创建目录
resp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
    CatalogName: "my-catalog",
    Comment:     "My catalog description",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created catalog ID: %d\n", resp.CatalogID)
```

## DeleteCatalog

删除指定的目录。

### 方法签名

```go
func (c *RawClient) DeleteCatalog(ctx context.Context, req *CatalogDeleteRequest, opts ...CallOption) (*CatalogDeleteResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| CatalogID | CatalogID | 是 | 要删除的目录 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| CatalogID | CatalogID | 已删除的目录 ID |

### 示例

```go
ctx := context.Background()

resp, err := client.DeleteCatalog(ctx, &sdk.CatalogDeleteRequest{
    CatalogID: 123,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Deleted catalog ID: %d\n", resp.CatalogID)
```

## UpdateCatalog

更新目录信息。

### 方法签名

```go
func (c *RawClient) UpdateCatalog(ctx context.Context, req *CatalogUpdateRequest, opts ...CallOption) (*CatalogUpdateResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| CatalogID | CatalogID | 是 | 要更新的目录 ID |
| CatalogName | string | 否 | 新的目录名称 |
| Comment | string | 否 | 新的目录描述/备注 |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| CatalogID | CatalogID | 已更新的目录 ID |

### 示例

```go
ctx := context.Background()

resp, err := client.UpdateCatalog(ctx, &sdk.CatalogUpdateRequest{
    CatalogID:   123,
    CatalogName: "updated-catalog-name",
    Comment:     "Updated description",
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Updated catalog ID: %d\n", resp.CatalogID)
```

## GetCatalog

获取指定目录的详细信息。

### 方法签名

```go
func (c *RawClient) GetCatalog(ctx context.Context, req *CatalogInfoRequest, opts ...CallOption) (*CatalogInfoResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| CatalogID | CatalogID | 是 | 目录 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| CatalogID | CatalogID | 目录 ID |
| CatalogName | string | 目录名称 |
| Comment | string | 目录描述/备注 |
| DatabaseCount | int | 数据库数量 |
| TableCount | int | 表数量 |
| VolumeCount | int | 卷数量 |
| FileCount | int | 文件数量 |
| Reserved | bool | 是否保留 |
| CreatedAt | string | 创建时间 |
| CreatedBy | string | 创建者 |
| UpdatedAt | string | 更新时间 |
| UpdatedBy | string | 更新者 |

### 示例

```go
ctx := context.Background()

resp, err := client.GetCatalog(ctx, &sdk.CatalogInfoRequest{
    CatalogID: 123,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Catalog: %s\n", resp.CatalogName)
fmt.Printf("Databases: %d, Tables: %d, Volumes: %d, Files: %d\n",
    resp.DatabaseCount, resp.TableCount, resp.VolumeCount, resp.FileCount)
```

## ListCatalogs

列出所有目录。

### 方法签名

```go
func (c *RawClient) ListCatalogs(ctx context.Context, opts ...CallOption) (*CatalogListResponse, error)
```

### 请求参数

无需参数。

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| List | []CatalogResponse | 目录列表 |

### 示例

```go
ctx := context.Background()

resp, err := client.ListCatalogs(ctx)
if err != nil {
    log.Fatal(err)
}

for _, catalog := range resp.List {
    fmt.Printf("Catalog ID: %d, Name: %s\n", catalog.CatalogID, catalog.CatalogName)
}
```

## GetCatalogTree

获取目录树结构（包含目录、数据库、表、卷的层级关系）。

### 方法签名

```go
func (c *RawClient) GetCatalogTree(ctx context.Context, opts ...CallOption) (*CatalogTreeResponse, error)
```

### 请求参数

无需参数。

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| Tree | []TreeNode | 目录树节点列表 |

**TreeNode 结构**:
- `ID`: 节点 ID
- `Name`: 节点名称
- `Type`: 节点类型（catalog, database, table, volume）
- `Children`: 子节点列表

### 示例

```go
ctx := context.Background()

resp, err := client.GetCatalogTree(ctx)
if err != nil {
    log.Fatal(err)
}

// 遍历目录树
for _, node := range resp.Tree {
    fmt.Printf("Type: %s, ID: %s, Name: %s\n", node.Type, node.ID, node.Name)
    // 递归处理子节点...
}
```

## GetCatalogRefList

获取目录的引用列表。

### 方法签名

```go
func (c *RawClient) GetCatalogRefList(ctx context.Context, req *CatalogRefListRequest, opts ...CallOption) (*CatalogRefListResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| CatalogID | CatalogID | 是 | 目录 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| RefList | []string | 引用列表 |

### 示例

```go
ctx := context.Background()

resp, err := client.GetCatalogRefList(ctx, &sdk.CatalogRefListRequest{
    CatalogID: 123,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("References: %v\n", resp.RefList)
```

## 完整示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/matrixorigin/moi-go-sdk/sdk"
)

func main() {
    client, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // 1. 创建目录
    createResp, err := client.CreateCatalog(ctx, &sdk.CatalogCreateRequest{
        CatalogName: "my-catalog",
        Comment:     "My catalog",
    })
    if err != nil {
        log.Fatal(err)
    }
    catalogID := createResp.CatalogID
    fmt.Printf("Created catalog: %d\n", catalogID)
    
    // 2. 获取目录信息
    infoResp, err := client.GetCatalog(ctx, &sdk.CatalogInfoRequest{
        CatalogID: catalogID,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Catalog name: %s\n", infoResp.CatalogName)
    
    // 3. 更新目录
    _, err = client.UpdateCatalog(ctx, &sdk.CatalogUpdateRequest{
        CatalogID:   catalogID,
        CatalogName: "updated-catalog",
        Comment:     "Updated description",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. 列出所有目录
    listResp, err := client.ListCatalogs(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Total catalogs: %d\n", len(listResp.List))
    
    // 5. 获取目录树
    treeResp, err := client.GetCatalogTree(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Tree nodes: %d\n", len(treeResp.Tree))
    
    // 6. 删除目录
    _, err = client.DeleteCatalog(ctx, &sdk.CatalogDeleteRequest{
        CatalogID: catalogID,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Catalog deleted")
}
```

## 错误处理

所有方法都可能返回以下错误：

- `ErrNilRequest`: 请求参数为 nil
- `*APIError`: API 业务错误（如目录不存在、名称冲突等）
- `*HTTPError`: HTTP 网络错误

```go
resp, err := client.CreateCatalog(ctx, req)
if err != nil {
    if apiErr, ok := err.(*sdk.APIError); ok {
        fmt.Printf("API Error: %s (code: %s)\n", apiErr.Message, apiErr.Code)
    } else {
        log.Fatal(err)
    }
}
```

