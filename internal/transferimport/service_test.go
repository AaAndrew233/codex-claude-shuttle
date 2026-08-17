package transferimport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/bundle"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/claude"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codexreconcile"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/handoff"
)

func TestClaudeNativeImportRewritesCWDAndVerifiesReadback(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000601"
	bundlePath := createClaudeBundle(t, id)
	targetRoot := t.TempDir()
	targetProject := filepath.Join(t.TempDir(), "目标项目")
	if err := os.MkdirAll(targetProject, 0o700); err != nil {
		t.Fatal(err)
	}
	service := testService()
	inspection, err := service.Inspect(bundlePath, "claude", targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.TransferImportRequest{
		BundlePath: bundlePath, SourceTool: "claude", TargetTool: "claude", TargetRoot: targetRoot, ConflictPolicy: ConflictSkip,
		Mappings: []domain.TransferProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: targetProject}},
	})
	if err != nil || !preflight.CanExecute {
		t.Fatalf("unexpected preflight: %+v %v", preflight, err)
	}
	result, err := service.Execute(context.Background(), preflight.PlanToken)
	if err != nil {
		t.Fatal(err)
	}
	if result.Written != 1 || !result.FilesVerified || result.Discovered != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	destination := filepath.Join(claude.ProjectsRoot(targetRoot), claude.EncodeProjectDirectory(targetProject), id+".jsonl")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	session, err := handoff.FromClaude(data)
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != id || session.CWD != targetProject || len(session.Turns) != 2 || session.Turns[0].Text != "Claude 原生问题" {
		t.Fatalf("unexpected native output: %s", data)
	}
}

func TestCodexToClaudeImportCreatesNativeClaudeTranscript(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000602"
	bundlePath := createCodexBundle(t, id)
	targetRoot := t.TempDir()
	targetProject := filepath.Join(t.TempDir(), "目标项目")
	if err := os.MkdirAll(targetProject, 0o700); err != nil {
		t.Fatal(err)
	}
	service := testService()
	inspection, err := service.Inspect(bundlePath, "claude", targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.TransferImportRequest{
		BundlePath: bundlePath, SourceTool: "codex", TargetTool: "claude", TargetRoot: targetRoot, ConflictPolicy: ConflictSkip,
		Mappings: []domain.TransferProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: targetProject}},
	})
	if err != nil || !preflight.CanExecute {
		t.Fatalf("unexpected preflight: %+v %v", preflight, err)
	}
	result, err := service.Execute(context.Background(), preflight.PlanToken)
	if err != nil {
		t.Fatal(err)
	}
	outputID, _ := handoff.TargetID("codex", id, "claude")
	destination := filepath.Join(claude.ProjectsRoot(targetRoot), claude.EncodeProjectDirectory(targetProject), outputID+".jsonl")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	session, err := handoff.FromClaude(data)
	if err != nil || len(session.Turns) != 2 || result.Written != 1 {
		t.Fatalf("unexpected converted Claude session: %+v %+v %v", session, result, err)
	}
}

func TestClaudeToCodexImportCreatesDiscoverableCodexRollout(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000603"
	bundlePath := createClaudeBundle(t, id)
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(targetRoot, ".codex-global-state.json"), []byte(`{"local-projects":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	targetProject := filepath.Join(t.TempDir(), "目标项目")
	if err := os.MkdirAll(targetProject, 0o700); err != nil {
		t.Fatal(err)
	}
	service := testService()
	inspection, err := service.Inspect(bundlePath, "codex", targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := service.Prepare(domain.TransferImportRequest{
		BundlePath: bundlePath, SourceTool: "claude", TargetTool: "codex", TargetRoot: targetRoot, ConflictPolicy: ConflictSkip,
		Mappings: []domain.TransferProjectMapping{{SourceKey: inspection.Sources[0].Key, TargetDirectory: targetProject}},
	})
	if err != nil || !preflight.CanExecute {
		t.Fatalf("unexpected preflight: %+v %v", preflight, err)
	}
	result, err := service.Execute(context.Background(), preflight.PlanToken)
	if err != nil {
		t.Fatal(err)
	}
	var rollout []byte
	err = filepath.WalkDir(filepath.Join(targetRoot, "sessions"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr == nil && !entry.IsDir() && strings.HasSuffix(path, ".jsonl") {
			rollout, walkErr = os.ReadFile(path)
		}
		return walkErr
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := handoff.FromCodex(rollout)
	if err != nil || len(session.Turns) != 2 || result.Discovered != 1 {
		t.Fatalf("unexpected converted Codex session: %+v %+v %v", session, result, err)
	}
}

func TestPrepareBlocksWhenTargetToolIsRunning(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000604"
	bundlePath := createClaudeBundle(t, id)
	targetRoot := t.TempDir()
	service := testService()
	service.running = func(string) (bool, error) { return true, nil }
	preflight, err := service.Prepare(domain.TransferImportRequest{BundlePath: bundlePath, SourceTool: "claude", TargetTool: "claude", TargetRoot: targetRoot})
	if err != nil || !preflight.ToolRunning || preflight.CanExecute {
		t.Fatalf("unexpected running-state preflight: %+v %v", preflight, err)
	}
}

func TestPrepareRejectsSymlinkedRecoveryRoot(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000605"
	bundlePath := createClaudeBundle(t, id)
	targetRoot := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(targetRoot, ".conversation-transfer-recovery")); err != nil {
		t.Fatal(err)
	}
	service := testService()
	_, err := service.Prepare(domain.TransferImportRequest{BundlePath: bundlePath, SourceTool: "claude", TargetTool: "claude", TargetRoot: targetRoot})
	if err == nil || !strings.Contains(err.Error(), "恢复目录") {
		t.Fatalf("expected unsafe recovery root rejection, got %v", err)
	}
}

func testService() *Service {
	service := NewService()
	service.running = func(string) (bool, error) { return false, nil }
	service.reconcile = func(_ context.Context, options codexreconcile.Options) (codexreconcile.Result, error) {
		return codexreconcile.Result{Verified: append([]string{}, options.ThreadIDs...)}, nil
	}
	return service
}

func createClaudeBundle(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, id+".jsonl")
	data := `{"type":"user","sessionId":"` + id + `","cwd":"/tmp/claude-source","timestamp":"2026-08-15T08:00:00Z","message":{"role":"user","content":"Claude 原生问题"}}` + "\n" +
		`{"type":"assistant","sessionId":"` + id + `","cwd":"/tmp/claude-source","timestamp":"2026-08-15T08:01:00Z","message":{"role":"assistant","content":[{"type":"text","text":"Claude 最终回复"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "claude.zip")
	summary := domain.ConversationSummary{ID: id, Title: "Claude 原生问题", UpdatedAt: "2026-08-15T08:01:00Z", SourceProject: "claude-source", SourceDirectory: "/tmp/claude-source"}
	if _, err := bundle.WriteTranscripts(destination, "claude", []bundle.TranscriptSource{{Conversation: summary, Path: path, FileName: filepath.Base(path)}}); err != nil {
		t.Fatal(err)
	}
	return destination
}

func createCodexBundle(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "rollout-2026-08-15T08-00-00-"+id+".jsonl")
	data := `{"timestamp":"2026-08-15T08:00:00Z","type":"session_meta","payload":{"id":"` + id + `","cwd":"/tmp/codex-source"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"Codex 原生问题"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"agent_message","message":"Codex 最终回复"}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "codex.zip")
	summary := domain.ConversationSummary{ID: id, Title: "Codex 原生问题", UpdatedAt: "2026-08-15T08:01:00Z", SourceProject: "codex-source", SourceDirectory: "/tmp/codex-source"}
	if _, err := bundle.WriteRollouts(destination, []bundle.RolloutSource{{Conversation: summary, Path: path, FileName: filepath.Base(path)}}); err != nil {
		t.Fatal(err)
	}
	return destination
}
