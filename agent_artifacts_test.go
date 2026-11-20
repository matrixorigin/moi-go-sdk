package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 验证成果版本、来源过滤和批处理的逐项结果不被通用 JSON 封装丢失。
func TestAgentArtifactProductMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.Method + " " + r.URL.EscapedPath() {
		case "GET /newmoi/workspaces/ws%2F1/agent-artifacts":
			if r.URL.Query().Get("keyword") != "report" || r.URL.Query().Get("page") != "2" {
				t.Errorf("artifact filters: %s", r.URL.RawQuery)
			}
			data = map[string]any{"items": []any{map[string]any{"artifact_id": "artifact/1", "version": 2, "file_id": "file-1", "generated_at": "2026-11-10T00:00:00Z"}}, "agent_sources": []any{map[string]any{"agent_id": "agent-1", "agent_workspace_id": "system"}}, "creator_sources": []any{map[string]any{"created_by": "user-1", "creator_name": "Alice"}}, "total": 3, "page": 2, "page_size": 1, "snapshot_at": "2026-11-10T00:01:00Z"}
		case "GET /newmoi/workspaces/ws%2F1/agent-artifacts/artifact%2F1/versions/2":
			data = map[string]any{"artifact_id": "artifact/1", "version": 2, "file_id": "file-1", "size": 9007199254740993, "creator_name": "Alice"}
		case "POST /newmoi/workspaces/ws%2F1/agent-artifacts/batch-delete", "POST /newmoi/workspaces/ws%2F1/agent-artifacts/resolve":
			var body struct {
				Items []struct {
					ArtifactID string `json:"artifact_id"`
					Version    int64  `json:"version"`
				} `json:"items"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Items) == 0 || body.Items[0].ArtifactID != "artifact/1" || body.Items[0].Version != 2 {
				t.Errorf("artifact refs: %+v, %v", body, err)
			}
			failed := []any{map[string]any{"artifact_id": "missing", "version": 1, "reason": "permission_denied"}}
			if r.URL.Path == "/newmoi/workspaces/ws/1/agent-artifacts/resolve" {
				data = map[string]any{"items": []any{map[string]any{"artifact_id": "artifact/1", "version": 2, "file_id": "file-1", "placement_id": 9, "volume_id": 7}}, "failed": failed}
			} else {
				data = map[string]any{"succeeded": []any{map[string]any{"artifact_id": "artifact/1", "version": 2}}, "failed": failed}
			}
		case "POST /newmoi/workspaces/ws%2F1/agent-artifacts/backfill":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["dry_run"] != true || body["limit"] != float64(4) {
				t.Errorf("backfill body: %#v, %v", body, err)
			}
			data = map[string]any{"scanned": 4, "inserted": 2, "skipped": 1, "failed": 1, "dry_run": true}
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected route", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "OK", "data": data})
	}))
	t.Cleanup(server.Close)
	client, err := New(server.URL+"/newmoi", WithAPIKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	agents := client.Agents("ws/1")
	list, err := agents.Artifacts(t.Context(), WithQueryParam("keyword", "report"), WithQueryParam("page", "2"))
	if err != nil || list.GetItems()[0].GetArtifactId() != "artifact/1" || list.GetCreatorSources()[0].GetCreatorName() != "Alice" || list.GetPage() != 2 {
		t.Fatalf("artifacts: %v, %v", list, err)
	}
	artifact, err := agents.Artifact(t.Context(), "artifact/1", 2)
	if err != nil || artifact.GetSize() != 9007199254740993 || artifact.GetVersion() != 2 {
		t.Fatalf("artifact: %v, %v", artifact, err)
	}
	refs := &AgentArtifactBatchRequest{Items: []*AgentArtifactRef{{ArtifactId: "artifact/1", Version: 2}, {ArtifactId: "missing", Version: 1}}}
	deleted, err := agents.DeleteArtifacts(t.Context(), refs)
	if err != nil || deleted.GetSucceeded()[0].GetVersion() != 2 || deleted.GetFailed()[0].GetReason() != "permission_denied" {
		t.Fatalf("delete: %v, %v", deleted, err)
	}
	resolved, err := agents.ResolveArtifacts(t.Context(), refs)
	if err != nil || resolved.GetItems()[0].GetFileId() != "file-1" || resolved.GetFailed()[0].GetArtifactId() != "missing" {
		t.Fatalf("resolve: %v, %v", resolved, err)
	}
	backfill, err := agents.BackfillArtifacts(t.Context(), &AgentArtifactBackfillRequest{DryRun: true, Limit: 4})
	if err != nil || !backfill.GetDryRun() || backfill.GetInserted() != 2 || backfill.GetFailed() != 1 {
		t.Fatalf("backfill: %v, %v", backfill, err)
	}
}
