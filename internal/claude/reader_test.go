package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanRootOnlyReturnsActiveSidebarConversations(t *testing.T) {
	root := t.TempDir()
	activeID := "00000000-0000-4000-8000-000000000111"
	archivedID := "00000000-0000-4000-8000-000000000112"
	orphanID := "00000000-0000-4000-8000-000000000113"
	sourceDirectory := filepath.Join(t.TempDir(), "alpha_project")

	writeSidebarFixture(t, root, "local_active", activeID, false, sourceDirectory, "侧边栏标题")
	writeSidebarFixture(t, root, "local_archived", archivedID, true, sourceDirectory, "已归档")
	writeTranscript(t, filepath.Join(root, "account", "organization", "local_deleted", ".claude", projectsDirectory, "orphan"), orphanID, sourceDirectory)

	scan, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Source != "claude-sidebar-read-only" || scan.Partial || scan.SkippedFiles != 0 {
		t.Fatalf("unexpected scan metadata: %+v", scan)
	}
	transcriptPath := filepath.Join(root, "account", "organization", "local_active", ".claude", projectsDirectory, EncodeProjectDirectory(sourceDirectory), activeID+".jsonl")
	transcriptInfo, err := os.Stat(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if scan.ScannedBytes <= transcriptInfo.Size() {
		t.Fatalf("scan byte limit must include sidebar metadata: got %d", scan.ScannedBytes)
	}
	if len(scan.Projects) != 1 || len(scan.Projects[0].Conversations) != 1 {
		t.Fatalf("unexpected scan: %+v", scan)
	}
	conversation := scan.Projects[0].Conversations[0]
	if conversation.ID != activeID || conversation.Title != "侧边栏标题" || conversation.UserMessage != "实现导入流程" || conversation.FinalReply != "已经完成。" {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
	if conversation.SourceProject != "alpha_project" || conversation.SourceDirectory != "" {
		t.Fatalf("unexpected project projection: %+v", conversation)
	}
}

func TestScanRootFailsClosedForMissingArchiveState(t *testing.T) {
	root := t.TempDir()
	organization := filepath.Join(root, "account", "organization")
	if err := os.MkdirAll(organization, 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := map[string]any{
		"sessionId":    "local_invalid",
		"cliSessionId": "00000000-0000-4000-8000-000000000114",
	}
	writeJSON(t, filepath.Join(organization, "local_invalid.json"), metadata)

	scan, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if !scan.Partial || scan.SkippedFiles != 1 || len(scan.Projects) != 0 {
		t.Fatalf("invalid metadata must remain invisible: %+v", scan)
	}
}

func TestScanRootFailsClosedForDuplicateCLISessionID(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000117"
	writeSidebarFixture(t, root, "local_duplicate_a", id, false, filepath.Join(t.TempDir(), "project-a"), "重复会话 A")
	writeSidebarFixture(t, root, "local_duplicate_b", id, false, filepath.Join(t.TempDir(), "project-b"), "重复会话 B")

	scan, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if !scan.Partial || scan.SkippedFiles != 2 || len(scan.Projects) != 0 {
		t.Fatalf("duplicate session identity must remain invisible: %+v", scan)
	}
}

func TestSelectTranscriptsRechecksSidebarState(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000115"
	metadataPath := writeSidebarFixture(t, root, "local_state_change", id, false, filepath.Join(t.TempDir(), "project"), "状态变化")
	if _, err := ScanRoot(root); err != nil {
		t.Fatal(err)
	}
	metadata := readJSONMap(t, metadataPath)
	metadata["isArchived"] = true
	writeJSON(t, metadataPath, metadata)

	if _, err := SelectTranscripts(root, []string{id}); err == nil || !strings.Contains(err.Error(), "已归档、删除") {
		t.Fatalf("expected archived selection to be rejected, got %v", err)
	}
}

func TestSelectTranscriptsRechecksDeletedSidebarMetadata(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000118"
	metadataPath := writeSidebarFixture(t, root, "local_deleted_after_scan", id, false, filepath.Join(t.TempDir(), "project"), "已删除")
	if _, err := ScanRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(metadataPath); err != nil {
		t.Fatal(err)
	}

	if _, err := SelectTranscripts(root, []string{id}); err == nil || !strings.Contains(err.Error(), "已归档、删除") {
		t.Fatalf("expected deleted selection to be rejected, got %v", err)
	}
}

func TestSelectTranscriptsRejectsSymlinkedSidebarTranscript(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000116"
	sessionID := "local_symlink"
	organization := filepath.Join(root, "account", "organization")
	project := filepath.Join(organization, sessionID, ".claude", projectsDirectory, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := sidebarSessionMetadata{SessionID: sessionID, CLISessionID: id, Title: "链接会话", CWD: "/tmp/project", LastActivityAt: 1_786_780_800_000, IsArchived: boolPointer(false)}
	writeJSON(t, filepath.Join(organization, sessionID+".json"), metadata)
	target := filepath.Join(t.TempDir(), id+".jsonl")
	if err := os.WriteFile(target, []byte(`{"type":"user","sessionId":"`+id+`","cwd":"/tmp/project","message":{"content":"hello"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(project, id+".jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := SelectTranscripts(root, []string{id}); err == nil {
		t.Fatal("expected symlinked transcript to be rejected")
	}
}

func TestEncodeProjectDirectory(t *testing.T) {
	if got := EncodeProjectDirectory("/tmp/Alpha_project v2"); got != "-tmp-Alpha-project-v2" {
		t.Fatalf("unexpected encoded directory: %s", got)
	}
}

func writeSidebarFixture(t *testing.T, root, sessionID, cliSessionID string, archived bool, sourceDirectory, title string) string {
	t.Helper()
	organization := filepath.Join(root, "account", "organization")
	project := filepath.Join(organization, sessionID, ".claude", projectsDirectory, EncodeProjectDirectory(sourceDirectory))
	writeTranscript(t, project, cliSessionID, sourceDirectory)
	metadata := sidebarSessionMetadata{
		SessionID:           sessionID,
		CLISessionID:        cliSessionID,
		Title:               title,
		CWD:                 sourceDirectory,
		UserSelectedFolders: []string{sourceDirectory},
		LastActivityAt:      1_786_780_800_000,
		IsArchived:          boolPointer(archived),
	}
	metadataPath := filepath.Join(organization, sessionID+".json")
	writeJSON(t, metadataPath, metadata)
	return metadataPath
}

func writeTranscript(t *testing.T, project, id, sourceDirectory string) {
	t.Helper()
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	records := []map[string]any{
		{"type": "user", "sessionId": id, "cwd": sourceDirectory, "timestamp": "2026-08-15T08:00:00Z", "message": map[string]any{"role": "user", "content": "实现导入流程"}},
		{"type": "assistant", "sessionId": id, "cwd": sourceDirectory, "timestamp": "2026-08-15T08:01:00Z", "message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "thinking", "thinking": "内部推理"}, {"type": "text", "text": "已经完成。"}}}},
		{"type": "assistant", "isSidechain": true, "sessionId": id, "cwd": sourceDirectory, "timestamp": "2026-08-15T08:02:00Z", "message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "子代理内容"}}}},
	}
	var data []byte
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, line...)
		data = append(data, '\n')
	}
	if err := os.WriteFile(filepath.Join(project, id+".jsonl"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func boolPointer(value bool) *bool {
	return &value
}
