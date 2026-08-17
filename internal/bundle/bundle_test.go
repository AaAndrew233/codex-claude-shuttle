package bundle

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

func TestWriteAndValidate(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "transfer.zip")
	scan := codex.SyntheticScan()
	conversations := scan.Projects[0].Conversations
	result, err := Write(destination, conversations)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IntegrityValid || result.ConversationCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	validation, err := Validate(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.Valid || validation.ProjectCount != 1 || validation.ConversationCount != 2 {
		t.Fatalf("unexpected validation: %+v", validation)
	}
	conversations, _, err = ReadConversations(destination)
	if err != nil || len(conversations) != 2 {
		t.Fatalf("expected readable conversations, got %d, %v", len(conversations), err)
	}
}

func TestWriteRolloutsPreservesCompleteLargeConversation(t *testing.T) {
	root := t.TempDir()
	rolloutPath := filepath.Join(root, "rollout-large.jsonl")
	rollout := bytes.Repeat([]byte("完整会话数据\n"), 1_000_000)
	if err := os.WriteFile(rolloutPath, rollout, 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "完整迁移包.zip")
	source := RolloutSource{
		Conversation: domain.ConversationSummary{ID: "large-conversation", Title: "大对话", SourceProject: "测试项目"},
		Path:         rolloutPath,
		FileName:     "rollout-large.jsonl",
	}
	result, err := WriteRollouts(destination, []RolloutSource{source})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConversationCount != 1 || !result.IntegrityValid {
		t.Fatalf("unexpected result: %+v", result)
	}
	validation, err := Validate(destination)
	if err != nil || !validation.Valid {
		t.Fatalf("expected valid bundle: %+v, %v", validation, err)
	}
	reader, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var restored []byte
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "rollouts/") {
			restored, err = readZipFile(file)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if !bytes.Equal(restored, rollout) {
		t.Fatal("complete rollout bytes were not preserved")
	}
}

func TestValidateRejectsTamperedEntry(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "transfer.zip")
	scan := codex.SyntheticScan()
	if _, err := Write(destination, scan.Projects[0].Conversations[:1]); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(t.TempDir(), "tampered.zip")
	output, err := os.Create(tampered)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(output)
	for _, file := range reader.File {
		data, err := readZipFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == "conversations/conv-market-brief.json" {
			data = []byte(`{"tampered":true}`)
		}
		w, err := zw.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	reader.Close()
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(tampered); err == nil {
		t.Fatal("expected tampered bundle to fail")
	}
}

func TestValidateRejectsHiddenArchiveEntry(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "transfer.zip")
	scan := codex.SyntheticScan()
	if _, err := Write(destination, scan.Projects[0].Conversations[:1]); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(t.TempDir(), "hidden-entry.zip")
	output, err := os.Create(tampered)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(output)
	for _, file := range reader.File {
		data, readErr := readZipFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		writer, createErr := zw.Create(file.Name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := writer.Write(data); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	hidden, err := zw.Create("conversations/.hidden.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hidden.Write([]byte("hidden")); err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(tampered); err == nil || !strings.Contains(err.Error(), "隐藏条目") {
		t.Fatalf("expected hidden entry rejection, got %v", err)
	}
}

func TestInspectRolloutsRejectsLegacySummaryOnlyBundle(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "summary-only.zip")
	conversation := domain.ConversationSummary{ID: "00000000-0000-4000-8000-000000000303", Title: "旧版摘要", SourceProject: "测试项目"}
	if _, err := Write(destination, []domain.ConversationSummary{conversation}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := InspectRollouts(destination); err == nil || !strings.Contains(err.Error(), "不包含可恢复的完整 Codex 会话") {
		t.Fatalf("expected summary-only package rejection, got %v", err)
	}
}

func TestWriteAndInspectClaudeTranscripts(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000404"
	transcriptPath := filepath.Join(root, id+".jsonl")
	data := []byte(`{"type":"user","sessionId":"` + id + `","cwd":"/tmp/claude-project","timestamp":"2026-08-15T08:00:00Z","message":{"role":"user","content":"hello"}}` + "\n")
	if err := os.WriteFile(transcriptPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "claude-transfer.zip")
	source := TranscriptSource{Conversation: domain.ConversationSummary{ID: id, Title: "hello", UpdatedAt: "2026-08-15T08:00:00Z", SourceProject: "claude-project", SourceDirectory: "/tmp/claude-project"}, Path: transcriptPath, FileName: filepath.Base(transcriptPath)}
	if _, err := WriteTranscripts(destination, "claude", []TranscriptSource{source}); err != nil {
		t.Fatal(err)
	}
	transcripts, validation, err := InspectTranscripts(destination)
	if err != nil {
		t.Fatal(err)
	}
	if validation.SourceTool != "claude" || len(transcripts) != 1 || transcripts[0].SessionID != id || transcripts[0].SourceDirectory != "/tmp/claude-project" {
		t.Fatalf("unexpected Claude package: %+v %+v", validation, transcripts)
	}
	restored, err := ReadTranscriptBytes(destination, transcripts[0], 1<<20)
	if err != nil || !bytes.Equal(restored, data) {
		t.Fatalf("unexpected restored transcript: %v", err)
	}
}

func TestWriteTranscriptsRejectsSymlinkSource(t *testing.T) {
	root := t.TempDir()
	id := "00000000-0000-4000-8000-000000000405"
	target := filepath.Join(root, "target.jsonl")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, id+".jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, err := WriteTranscripts(filepath.Join(t.TempDir(), "unsafe.zip"), "claude", []TranscriptSource{{Conversation: domain.ConversationSummary{ID: id, SourceProject: "project"}, Path: link, FileName: filepath.Base(link)}})
	if err == nil || !strings.Contains(err.Error(), "安全的常规文件") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func readZipFile(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
