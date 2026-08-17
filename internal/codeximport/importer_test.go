package codeximport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/bundle"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codexreconcile"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

const testThreadID = "00000000-0000-4000-8000-000000000101"

func TestRealCodexImportWritesMappedRolloutAndVerifiesDiscovery(t *testing.T) {
	root, target := createCodexTarget(t)
	bundlePath := createRolloutBundle(t, "/source/device/project-alpha", "项目 Alpha")
	service := NewService()
	service.running = func() (bool, error) { return false, nil }
	service.reconcile = func(_ context.Context, options codexreconcile.Options) (codexreconcile.Result, error) {
		if options.CodexRoot != root || len(options.ThreadIDs) != 1 || options.ThreadIDs[0] != testThreadID {
			t.Fatalf("unexpected reconcile options: %+v", options)
		}
		return codexreconcile.Result{Version: "test", Verified: []string{testThreadID}}, nil
	}

	inspection, err := service.Inspect(bundlePath, root)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.ConversationCount != 1 || len(inspection.Sources) != 1 || len(inspection.Targets) != 1 {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
	request := domain.CodexImportRequest{BundlePath: bundlePath, CodexRoot: root, ConflictPolicy: ConflictSkip, Mappings: []domain.CodexProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetID: inspection.Targets[0].ID}}}
	preflight, err := service.Prepare(request)
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.CanExecute || preflight.CreateCount != 1 || preflight.PlanToken == "" {
		t.Fatalf("unexpected preflight: %+v", preflight)
	}
	result, err := service.Execute(context.Background(), preflight.PlanToken)
	if err != nil {
		t.Fatal(err)
	}
	if result.Written != 1 || result.Discovered != 1 || !result.FilesVerified || result.PendingDiscovery != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	destination := filepath.Join(root, "sessions", "2026", "08", "14", "rollout-2026-08-14T12-00-00-"+testThreadID+".jsonl")
	file, err := os.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	first, err := bufio.NewReader(file).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var wrapper struct {
		Payload struct {
			CWD string `json:"cwd"`
			ID  string `json:"id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(first, &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Payload.CWD != target || wrapper.Payload.ID != testThreadID {
		t.Fatalf("session meta was not mapped safely: %+v", wrapper.Payload)
	}
}

func TestRealCodexImportSkipsConflictByDefault(t *testing.T) {
	root, target := createCodexTarget(t)
	bundlePath := createRolloutBundle(t, "/source/project", "项目 Alpha")
	destination := filepath.Join(root, "sessions", "2026", "08", "14", "rollout-2026-08-14T12-00-00-"+testThreadID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("device copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService()
	service.running = func() (bool, error) { return false, nil }
	service.reconcile = func(_ context.Context, _ codexreconcile.Options) (codexreconcile.Result, error) {
		t.Fatal("reconcile must not run when every conflict is skipped")
		return codexreconcile.Result{}, nil
	}
	inspection, err := service.Inspect(bundlePath, root)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.CodexImportRequest{BundlePath: bundlePath, CodexRoot: root, Mappings: []domain.CodexProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: target}}})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.SkipCount != 1 || !preflight.CanExecute {
		t.Fatalf("unexpected conflict preflight: %+v", preflight)
	}
	result, err := service.Execute(context.Background(), preflight.PlanToken)
	if err != nil || result.Skipped != 1 {
		t.Fatalf("unexpected skip result: %+v, %v", result, err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "device copy" {
		t.Fatalf("existing conversation changed: %q, %v", data, err)
	}
}

func TestRealCodexImportFindsConflictByThreadIDEvenWhenFilenameDiffers(t *testing.T) {
	root, target := createCodexTarget(t)
	bundlePath := createRolloutBundle(t, "/source/project", "项目 Alpha")
	destination := filepath.Join(root, "sessions", "2025", "01", "02", "rollout-older-name-"+testThreadID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing thread"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService()
	service.running = func() (bool, error) { return false, nil }
	inspection, err := service.Inspect(bundlePath, root)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.CodexImportRequest{BundlePath: bundlePath, CodexRoot: root, Mappings: []domain.CodexProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: target}}})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.SkipCount != 1 || preflight.CreateCount != 0 {
		t.Fatalf("same thread ID must be treated as a conflict: %+v", preflight)
	}
}

func TestRealCodexImportReplacesConflictAndKeepsMappedRollout(t *testing.T) {
	root, target := createCodexTarget(t)
	bundlePath := createRolloutBundle(t, "/source/project", "项目 Alpha")
	destination := filepath.Join(root, "sessions", "2025", "01", "02", "rollout-older-name-"+testThreadID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing thread"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService()
	service.running = func() (bool, error) { return false, nil }
	service.reconcile = func(_ context.Context, _ codexreconcile.Options) (codexreconcile.Result, error) {
		return codexreconcile.Result{Verified: []string{testThreadID}}, nil
	}
	inspection, err := service.Inspect(bundlePath, root)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.CodexImportRequest{BundlePath: bundlePath, CodexRoot: root, ConflictPolicy: ConflictReplace, Mappings: []domain.CodexProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: target}}})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.ReplaceCount != 1 || !preflight.CanExecute {
		t.Fatalf("unexpected replacement preflight: %+v", preflight)
	}
	result, err := service.Execute(context.Background(), preflight.PlanToken)
	if err != nil {
		t.Fatal(err)
	}
	if result.Replaced != 1 || !result.FilesVerified {
		t.Fatalf("unexpected replacement result: %+v", result)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	var wrapper struct {
		Payload struct {
			CWD string `json:"cwd"`
			ID  string `json:"id"`
		} `json:"payload"`
	}
	first := bytes.SplitN(data, []byte("\n"), 2)[0]
	if err := json.Unmarshal(first, &wrapper); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("existing thread")) || wrapper.Payload.CWD != target || wrapper.Payload.ID != testThreadID {
		t.Fatalf("replacement did not contain mapped rollout: %s", data)
	}
}

func TestRecoverPendingRestoresInterruptedReplacement(t *testing.T) {
	root, _ := createCodexTarget(t)
	target := filepath.Join(root, "sessions", "2026", "08", "14", "rollout-test-"+testThreadID+".jsonl")
	journalRoot := filepath.Join(root, ".codex-transfer-recovery", "interrupted")
	backup := filepath.Join(journalRoot, "backups", "000000.bak")
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := recoveryJournal{Version: 1, CodexRoot: root, Entries: []recoveryEntry{{TargetPath: target, BackupPath: backup, Committed: true}}}
	if err := writeJournal(journalRoot, journal); err != nil {
		t.Fatal(err)
	}
	if err := recoverPending(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "original" {
		t.Fatalf("interrupted replacement was not restored: %q, %v", data, err)
	}
}

func TestRecoverPendingKeepsCompletedImport(t *testing.T) {
	root, _ := createCodexTarget(t)
	target := filepath.Join(root, "sessions", "2026", "08", "14", "rollout-test-"+testThreadID+".jsonl")
	journalRoot := filepath.Join(root, ".codex-transfer-recovery", "completed")
	backup := filepath.Join(journalRoot, "backups", "000000.bak")
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := recoveryJournal{Version: 1, CodexRoot: root, Completed: true, Entries: []recoveryEntry{{TargetPath: target, BackupPath: backup, Committed: true}}}
	if err := writeJournal(journalRoot, journal); err != nil {
		t.Fatal(err)
	}
	if err := recoverPending(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "new" {
		t.Fatalf("completed import was incorrectly rolled back: %q, %v", data, err)
	}
}

func TestRealCodexImportBlocksWhileCodexRuns(t *testing.T) {
	root, target := createCodexTarget(t)
	bundlePath := createRolloutBundle(t, "/source/project", "项目 Alpha")
	service := NewService()
	service.running = func() (bool, error) { return true, nil }
	inspection, err := service.Inspect(bundlePath, root)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.CodexImportRequest{BundlePath: bundlePath, CodexRoot: root, Mappings: []domain.CodexProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: target}}})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.CanExecute || !preflight.CodexRunning || !strings.Contains(preflight.Message, "退出 Codex") {
		t.Fatalf("running Codex was not blocked: %+v", preflight)
	}
}

func TestPrepareDoesNotRecoverPendingJournalWhileCodexRuns(t *testing.T) {
	root, _ := createCodexTarget(t)
	target := filepath.Join(root, "sessions", "2026", "08", "14", "rollout-test-"+testThreadID+".jsonl")
	journalRoot := filepath.Join(root, ".codex-transfer-recovery", "pending-while-running")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(journalRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("must remain"), 0o600); err != nil {
		t.Fatal(err)
	}
	journal := recoveryJournal{Version: 1, CodexRoot: root, Entries: []recoveryEntry{{TargetPath: target, Created: true, Committed: true}}}
	if err := writeJournal(journalRoot, journal); err != nil {
		t.Fatal(err)
	}
	service := NewService()
	service.running = func() (bool, error) { return true, nil }
	preflight, err := service.Prepare(domain.CodexImportRequest{CodexRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.CodexRunning || preflight.CanExecute {
		t.Fatalf("running Codex must block before recovery: %+v", preflight)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "must remain" {
		t.Fatalf("pending recovery changed files while Codex was running: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(journalRoot, "journal.json")); err != nil {
		t.Fatalf("pending recovery journal was consumed while Codex was running: %v", err)
	}
}

func createCodexTarget(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "project-alpha")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{"local-projects": map[string]any{"target-alpha": map[string]any{"id": "target-alpha", "name": "目标 Alpha", "rootPaths": []string{target}}}, "sidebar-project-thread-orders": map[string]any{}}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex-global-state.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, target
}

func createRolloutBundle(t *testing.T, sourceDirectory, sourceProject string) string {
	t.Helper()
	rolloutPath := filepath.Join(t.TempDir(), "rollout-2026-08-14T12-00-00-"+testThreadID+".jsonl")
	content := `{"timestamp":"2026-08-14T12:00:00Z","type":"session_meta","payload":{"id":"` + testThreadID + `","timestamp":"2026-08-14T12:00:00Z","cwd":"` + sourceDirectory + `","originator":"codex","cli_version":"test"}}
{"timestamp":"2026-08-14T12:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"合成问题"}}
{"timestamp":"2026-08-14T12:00:02Z","type":"event_msg","payload":{"type":"agent_message","message":"合成回答"}}
`
	if err := os.WriteFile(rolloutPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "real-transfer.zip")
	_, err := bundle.WriteRollouts(destination, []bundle.RolloutSource{{Conversation: domain.ConversationSummary{ID: testThreadID, Title: "合成问题", UpdatedAt: "2026-08-14T12:00:02Z", UserMessage: "合成问题", FinalReply: "合成回答", SourceProject: sourceProject}, Path: rolloutPath, FileName: filepath.Base(rolloutPath)}})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}
