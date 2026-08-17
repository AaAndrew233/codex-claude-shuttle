package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

func TestApplicationInfo(t *testing.T) {
	info := NewApp().ApplicationInfo()

	if info.Name != applicationName {
		t.Fatalf("unexpected application name: %q", info.Name)
	}
	if info.Version != applicationVersion {
		t.Fatalf("unexpected application version: %q", info.Version)
	}
}

func TestDefaultCodexRootIsUnderUserHome(t *testing.T) {
	root, err := NewApp().DefaultCodexRoot()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".codex")
	if root != want {
		t.Fatalf("unexpected default root: got %q want %q", root, want)
	}
}

func TestDefaultClaudeRootIsUnderUserHome(t *testing.T) {
	root, err := NewApp().DefaultClaudeRoot()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Join(home, ".claude") {
		t.Fatalf("unexpected Claude root: %q", root)
	}
}

func TestDefaultClaudeSidebarRootIsSeparateFromImportRoot(t *testing.T) {
	sidebarRoot, err := NewApp().DefaultClaudeSidebarRoot()
	if err != nil {
		t.Fatal(err)
	}
	importRoot, err := NewApp().DefaultClaudeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(sidebarRoot) || filepath.Base(sidebarRoot) != "local-agent-mode-sessions" || sidebarRoot == importRoot {
		t.Fatalf("unexpected Claude sidebar root: %q", sidebarRoot)
	}
}

func TestExportClaudeUsesSelectedNativeTranscript(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000701"
	sessionID := "local_00000000-0000-4000-8000-000000000702"
	organization := filepath.Join(root, "account", "organization")
	project := filepath.Join(organization, sessionID, ".claude", "projects", "-tmp-claude-project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"user","sessionId":"` + id + `","cwd":"/tmp/claude-project","timestamp":"2026-08-15T08:00:00Z","message":{"content":"真实 Claude 问题"}}` + "\n"
	if err := os.WriteFile(filepath.Join(project, id+".jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata := `{"sessionId":"` + sessionID + `","cliSessionId":"` + id + `","title":"真实 Claude 对话","cwd":"/tmp/claude-project","userSelectedFolders":["/tmp/claude-project"],"lastActivityAt":1786780800000,"isArchived":false}`
	if err := os.WriteFile(filepath.Join(organization, sessionID+".json"), []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "Claude-迁移包.zip")
	result, err := NewApp().ExportClaude(root, domain.ExportRequest{Destination: destination, ConversationIDs: []string{id}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConversationCount != 1 || !result.IntegrityValid {
		t.Fatalf("unexpected export result: %+v", result)
	}
}

func TestChooseExportFolderRequiresStartupContext(t *testing.T) {
	if _, err := NewApp().ChooseExportFolder(""); err == nil {
		t.Fatal("expected folder chooser to require a Wails startup context")
	}
}

func TestChooseImportFileRequiresStartupContext(t *testing.T) {
	if _, err := NewApp().ChooseImportFile(""); err == nil {
		t.Fatal("expected file chooser to require a Wails startup context")
	}
}

func TestOpenExportFolderRejectsMissingPath(t *testing.T) {
	if err := NewApp().OpenExportFolder(filepath.Join(t.TempDir(), "missing.zip")); err == nil {
		t.Fatal("expected missing export path error")
	}
}

func TestExportCodexUsesSelectedRealConversation(t *testing.T) {
	root := t.TempDir()
	threadID := "00000000-0000-0000-0000-000000000010"
	state := `{"local-projects":{"project-real":{"id":"project-real","name":"真实项目"}},"sidebar-project-thread-orders":{"project-real":{"threadIds":["` + threadID + `"]}}}`
	if err := os.WriteFile(filepath.Join(root, ".codex-global-state.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(root, "sessions", "2026", "08", "13")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	data := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"" + threadID + "\",\"cwd\":\"/synthetic/real-project\"}}\n" +
		"{\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"真实问题\"}}\n" +
		"{\"type\":\"event_msg\",\"payload\":{\"type\":\"agent_message\",\"message\":\"真实回答\"}}\n"
	if err := os.WriteFile(filepath.Join(sessions, "rollout-2026-08-13T10-00-00-"+threadID+".jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "真实迁移包.zip")
	result, err := NewApp().ExportCodex(root, domain.ExportRequest{Destination: destination, ConversationIDs: []string{threadID}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConversationCount != 1 || !result.IntegrityValid {
		t.Fatalf("unexpected export result: %+v", result)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatal(err)
	}
}

func TestExportCodexRejectsMissingSelection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".codex-global-state.json"), []byte(`{"sidebar-project-thread-orders":{"project":{"threadIds":[]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewApp().ExportCodex(root, domain.ExportRequest{Destination: filepath.Join(t.TempDir(), "empty.zip"), ConversationIDs: []string{"missing"}})
	if err == nil {
		t.Fatal("expected missing selection error")
	}
}
