# Database（数据库）接口

Database 是 Catalog 下的数据组织单元，用于管理表和卷。

## 接口列表

- [CreateDatabase](#createdatabase) - 创建数据库
- [DeleteDatabase](#deletedatabase) - 删除数据库
- [UpdateDatabase](#updatedatabase) - 更新数据库
- [GetDatabase](#getdatabase) - 获取数据库信息
- [ListDatabases](#listdatabases) - 列出目录下的所有数据库
- [GetDatabaseChildren](#getdatabasechildren) - 获取数据库的子项（表和卷）
- [GetDatabaseRefList](#getdatabasereflist) - 获取数据库引用列表

## CreateDatabase

在指定目录下创建新的数据库。

### 方法签名

```go
func (c *RawClient) CreateDatabase(ctx context.Context, req *DatabaseCreateRequest, opts ...CallOption) (*DatabaseCreateResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| DatabaseName | string | 是 | 数据库名称 |
| Comment | string | 否 | 数据库描述/备注 |
| CatalogID | CatalogID | 是 | 所属目录 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| DatabaseID | DatabaseID | 创建的数据库 ID |

### 示例

```go
ctx := context.Background()

resp, err := client.CreateDatabase(ctx, &sdk.DatabaseCreateRequest{
    DatabaseName: "my-database",
    Comment:      "My database description",
    CatalogID:    123,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created database ID: %d\n", resp.DatabaseID)
```

## DeleteDatabase

删除指定的数据库。

### 方法签名

```go
func (c *RawClient) DeleteDatabase(ctx context.Context, req *DatabaseDeleteRequest, opts ...CallOption) (*DatabaseDeleteResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| DatabaseID | DatabaseID | 是 | 要删除的数据库 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| DatabaseID | DatabaseID | 已删除的数据库 ID |

### 示例

```go
ctx := context.Background()

resp, err := client.DeleteDatabase(ctx, &sdk.DatabaseDeleteRequest{
    DatabaseID: 456,
})
if err != nil {
    log.Fatal(err)
}
```

## UpdateDatabase

更新数据库信息。

### 方法签名

```go
func (c *RawClient) UpdateDatabase(ctx context.Context, req *DatabaseUpdateRequest, opts ...CallOption) (*DatabaseUpdateResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| DatabaseID | DatabaseID | 是 | 要更新的数据库 ID |
| Comment | string | 否 | 新的数据库描述/备注 |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| DatabaseID | DatabaseID | 已更新的数据库 ID |

### 示例

```go
ctx := context.Background()

resp, err := client.UpdateDatabase(ctx, &sdk.DatabaseUpdateRequest{
    DatabaseID: 456,
    Comment:    "Updated description",
})
if err != nil {
    log.Fatal(err)
}
```

## GetDatabase

获取指定数据库的详细信息。

### 方法签名

```go
func (c *RawClient) GetDatabase(ctx context.Context, req *DatabaseInfoRequest, opts ...CallOption) (*DatabaseInfoResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| DatabaseID | DatabaseID | 是 | 数据库 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| DatabaseID | DatabaseID | 数据库 ID |
| DatabaseName | string | 数据库名称 |
| Comment | string | 数据库描述/备注 |
| CreatedAt | string | 创建时间 |
| UpdatedAt | string | 更新时间 |

### 示例

```go
ctx := context.Background()

resp, err := client.GetDatabase(ctx, &sdk.DatabaseInfoRequest{
    DatabaseID: 456,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Database: %s\n", resp.DatabaseName)
```

## ListDatabases

列出指定目录下的所有数据库。

### 方法签名

```go
func (c *RawClient) ListDatabases(ctx context.Context, req *DatabaseListRequest, opts ...CallOption) (*DatabaseListResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| CatalogID | CatalogID | 是 | 目录 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| List | []DatabaseResponse | 数据库列表 |

### 示例

```go
ctx := context.Background()

resp, err := client.ListDatabases(ctx, &sdk.DatabaseListRequest{
    CatalogID: 123,
})
if err != nil {
    log.Fatal(err)
}

for _, db := range resp.List {
    fmt.Printf("Database ID: %d, Name: %s\n", db.DatabaseID, db.DatabaseName)
}
```

## GetDatabaseChildren

获取数据库的子项（表和卷）。

### 方法签名

```go
func (c *RawClient) GetDatabaseChildren(ctx context.Context, req *DatabaseChildrenRequest, opts ...CallOption) (*DatabaseChildrenResponseData, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| DatabaseID | DatabaseID | 是 | 数据库 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| Tables | []TableResponse | 表列表 |
| Volumes | []VolumeChildrenResponse | 卷列表 |

### 示例

```go
ctx := context.Background()

resp, err := client.GetDatabaseChildren(ctx, &sdk.DatabaseChildrenRequest{
    DatabaseID: 456,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Tables: %d, Volumes: %d\n", len(resp.Tables), len(resp.Volumes))
```

## GetDatabaseRefList

获取数据库的引用列表。

### 方法签名

```go
func (c *RawClient) GetDatabaseRefList(ctx context.Context, req *DatabaseRefListRequest, opts ...CallOption) (*DatabaseRefListResponse, error)
```

### 请求参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| DatabaseID | DatabaseID | 是 | 数据库 ID |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| RefList | []string | 引用列表 |

### 示例

```go
ctx := context.Background()

resp, err := client.GetDatabaseRefList(ctx, &sdk.DatabaseRefListRequest{
    DatabaseID: 456,
})
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
    
    "github.com/matrixorigin/moi-go-sdk/sdk"
)

func main() {
    client, err := sdk.NewRawClient("https://api.example.com", "your-api-key")
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    catalogID := sdk.CatalogID(123)
    
    // 1. 创建数据库
    createResp, err := client.CreateDatabase(ctx, &sdk.DatabaseCreateRequest{
        DatabaseName: "my-database",
        Comment:      "My database",
        CatalogID:    catalogID,
    })
    if err != nil {
        log.Fatal(err)
    }
    databaseID := createResp.DatabaseID
    
    // 2. 获取数据库信息
    infoResp, err := client.GetDatabase(ctx, &sdk.DatabaseInfoRequest{
        DatabaseID: databaseID,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. 列出目录下的所有数据库
    listResp, err := client.ListDatabases(ctx, &sdk.DatabaseListRequest{
        CatalogID: catalogID,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. 获取数据库的子项
    childrenResp, err := client.GetDatabaseChildren(ctx, &sdk.DatabaseChildrenRequest{
        DatabaseID: databaseID,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. 更新数据库
    _, err = client.UpdateDatabase(ctx, &sdk.DatabaseUpdateRequest{
        DatabaseID: databaseID,
        Comment:    "Updated description",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // 6. 删除数据库
    _, err = client.DeleteDatabase(ctx, &sdk.DatabaseDeleteRequest{
        DatabaseID: databaseID,
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

