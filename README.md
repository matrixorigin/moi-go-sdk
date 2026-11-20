# MOI Product Go SDK

[简体中文](README_zh-CN.md)

Use the MatrixOne Intelligence (MOI) 5.x AI Studio product API from Go to manage workspaces, Catalog resources and files, workflows, knowledge bases, SQL, and agents. The server enforces workspace and role permissions.

## Requirements and installation

Use Go 1.25.7 or newer. The Product SDK module is `github.com/matrixorigin/moi-go-sdk/v5` and includes the generated product types.

Install [v5.2.3](https://github.com/matrixorigin/moi-go-sdk/releases/tag/v5.2.3) in your own Go module:

```sh
go get github.com/matrixorigin/moi-go-sdk/v5@v5.2.3
```

For other versions, choose a `v5.Y.Z` tag that matches your deployment from [GitHub Releases](https://github.com/matrixorigin/moi-go-sdk/releases). After installing, save the example below in your module and run `go run .`.

The [official SDK downloads page](https://docs.matrixorigin.cn/moi/en/5.0/developer/quickstart/downloads/ai-studio.html) also offers the older `0.1.0-r2` offline archive. If you use it, keep its bundled `go.work` and module layout together and follow the download page to verify its bundled example.

## Authentication and quickstart

Create an AI Studio personal access token (PAT) in the MOI console. The example reads the endpoint and token from the environment and passes them explicitly to the client. The SDK sends the raw token as `X-API-Key`, without a `Bearer` prefix. Use the hosted endpoint below or the full API endpoint and path prefix supplied by your deployment administrator. Send the token only to a trusted AI Studio product endpoint. Inject it through a protected environment and keep it out of source, shell history, and logs.

```sh
export PRODUCT_API_BASE_URL='https://api.moi.matrixorigin.cn/v5'
export PRODUCT_API_KEY='YOUR_PERSONAL_ACCESS_TOKEN'
```

This example binds a workspace by name and prints its ID, using the promoted `EnsureWorkspace` user-story entry. Once you have a workspace, continue with `WorkspaceHandle` for workspace-scoped tasks. Use `client.Workspace(workspaceID)` only when you already sourced the workspace id.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	sdk "github.com/matrixorigin/moi-go-sdk/v5"
)

func main() {
	client, err := sdk.NewWithPersonalAccessToken(
		os.Getenv("PRODUCT_API_BASE_URL"),
		os.Getenv("PRODUCT_API_KEY"),
	)
	if err != nil {
		log.Fatal(err)
	}
	workspace, err := client.EnsureWorkspace(context.Background(), "my-first-workspace")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(workspace.ID(), workspace.Name())
}
```

Low-level route-aligned services stay available for explicit control:

```go
result, err := client.Workspaces().List(context.Background())
if err != nil {
	log.Fatal(err)
}
for _, workspace := range result.GetWorkspaces() {
	fmt.Println(workspace.GetId(), workspace.GetName())
}
```

The PAT identifies your account. Workspace requests use the server's Default Role unless you select a role for that workspace with `sdk.WithWorkspaceCurrentRoleID` when constructing the client. Use an assigned role ID. The server validates the role and rejects invalid explicit selections.

## Compatibility and license

Go SDK 5.2.3 targets the AI Studio API (`/v5`) on MOI 5.2.3. For other MOI versions, use the build specified by your deployment documentation. Older offline packages may cover different capabilities.

The [legacy Catalog SDK](https://github.com/matrixorigin/moi-go-sdk/tree/legacy) targets MOI 4.0. To migrate to MOI 5.x, verify that you have a Product SDK build, then switch the API endpoint, authentication, and client methods and verify each business operation.

This directory is licensed under the [Apache License 2.0](LICENSE).

## Documentation

- [AI Studio SDK quickstart](https://docs.matrixorigin.cn/moi/en/5.0/developer/quickstart/ai-studio/sdk.html) and [API setup](https://docs.matrixorigin.cn/moi/en/5.0/developer/quickstart/ai-studio/http-api.html)
- [Tasks and API notes](#tasks-and-api-notes) for files, workflows, knowledge bases, and Agent A2A.

## Tasks and API notes

| Task | Entry point |
| --- | --- |
| Bind a workspace and select resources | `WorkspaceHandle` |
| Manage volumes and files | `VolumeHandle` / `FileHandle` |
| Deploy, run, and inspect workflows | `WorkflowHandle` / `WorkflowRunHandle` |
| Manage knowledge bases and semantic entries | `SemanticModelHandle` |
| Invoke agents and follow A2A tasks | `AgentHandle` / `SendText` |

- The SDK returns typed product results. `Error.Details` retains structured diagnostics from the server.
- Pass capability IDs to `CurrentPrincipalCapabilities` to check specific permissions. The server reports unrecognized IDs separately.
- `SQLService.DownloadQueryResult` streams the saved CSV result for a statement ID. Close the response stream after reading. [Run SQL queries](https://docs.matrixorigin.cn/moi/en/5.0/developer/sdk/ai-studio/%E6%95%B0%E6%8D%AE%E5%A4%84%E7%90%86/%E6%89%A7%E8%A1%8CSQL%E6%9F%A5%E8%AF%A2.html)
- A2A `message/send` responds at admission and may return a task in `submitted` or `working`. The server rejects `configuration.blocking: true` with HTTP 400, JSON-RPC `-32602`, and `BLOCKING_UNSUPPORTED`.
- `AgentHandle.SendText` and `AgentService.A2A` follow pending tasks with `tasks/resubscribe` and poll `tasks/get` every 2 seconds if streaming is unavailable. Handle the returned terminal state or `input-required` or `auth-required` status. `WithA2AWaitTimeout` defaults to 10 minutes for the whole follow, independent of per-request `WithTimeout`. After `ErrTimeout`, use `task_id` in `Error.Details` to query the same task.
- Set `A2ARequest.NoWait` on an explicit A2A request to return at admission. The setting controls client-side following. Keep the returned task ID. [Invoke agents and track tasks](https://docs.matrixorigin.cn/moi/en/5.0/developer/sdk/ai-studio/%E6%99%BA%E8%83%BD%E4%BD%93/%E8%B0%83%E7%94%A8%E6%99%BA%E8%83%BD%E4%BD%93%E5%B9%B6%E8%B7%9F%E8%B8%AA%E4%BB%BB%E5%8A%A1.html)
- `MemoryService` and `ConsumerMemoryHandle` refer to the legacy Consumer Memory/Profile API. Current MOI product deployments do not expose those endpoints.
