package importexec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

func testConversations() []domain.ConversationSummary {
	scan := codex.SyntheticScan()
	return scan.Projects[0].Conversations
}

func TestExecuteCreatesAndReadsBackSyntheticConversations(t *testing.T) {
	home, err := codex.NewSyntheticHome(filepath.Join(t.TempDir(), "home-b"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := home.CreateProject("市场调研")
	if err != nil {
		t.Fatal(err)
	}
	conversations := testConversations()
	plan, err := BuildPlan("/tmp/fixture.zip", conversations, map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), plan, Options{CodexClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Written != 2 || result.Replaced != 0 || result.Skipped != 0 || result.Recovered {
		t.Fatalf("unexpected result: %+v", result)
	}
	readBack, err := home.ReadProject(target)
	if err != nil || len(readBack) != 2 {
		t.Fatalf("read back failed: %d, %v", len(readBack), err)
	}
	if _, err := os.Stat(result.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal should be cleaned, got %v", err)
	}
}

func TestExecuteSkipsIdenticalAndReplacesWhenRequested(t *testing.T) {
	home, err := codex.NewSyntheticHome(filepath.Join(t.TempDir(), "home-b"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := home.CreateProject("市场调研")
	if err != nil {
		t.Fatal(err)
	}
	conversation := testConversations()[0]
	plan, err := BuildPlan("/tmp/fixture.zip", []domain.ConversationSummary{conversation}, map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), plan, Options{CodexClosed: true}); err != nil {
		t.Fatal(err)
	}
	plan, err = BuildPlan("/tmp/fixture.zip", []domain.ConversationSummary{conversation}, map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), plan, Options{CodexClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 || result.Written != 0 {
		t.Fatalf("expected identical skip: %+v", result)
	}
	conversation.FinalReply = "替换后的合成回复"
	plan, err = BuildPlan("/tmp/fixture.zip", []domain.ConversationSummary{conversation}, map[string]domain.TargetProject{"市场调研": target}, ConflictReplace)
	if err != nil {
		t.Fatal(err)
	}
	result, err = Execute(context.Background(), plan, Options{CodexClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Replaced != 1 || result.Recovered {
		t.Fatalf("expected replacement: %+v", result)
	}
}

func TestExecuteRecoversAfterInterruptedBatch(t *testing.T) {
	home, err := codex.NewSyntheticHome(filepath.Join(t.TempDir(), "home-b"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := home.CreateProject("市场调研")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan("/tmp/fixture.zip", testConversations(), map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), plan, Options{CodexClosed: true, BeforeItem: func(index int) error {
		if index == 1 {
			return errors.New("模拟中断")
		}
		return nil
	}})
	if err == nil || !result.Recovered {
		t.Fatalf("expected recovery, got %+v, %v", result, err)
	}
	entries, err := os.ReadDir(filepath.Join(target.Directory, "conversations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("recovery left conversation files behind: %d %v", len(entries), names)
	}
}

func TestExecuteRejectsOpenCodexExpiredAndChangedPlan(t *testing.T) {
	home, err := codex.NewSyntheticHome(filepath.Join(t.TempDir(), "home-b"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := home.CreateProject("市场调研")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan("/tmp/fixture.zip", testConversations()[:1], map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), plan, Options{}); err == nil {
		t.Fatal("expected open Codex to be rejected")
	}
	plan.ExpiresAt = time.Now().Add(-time.Minute)
	if _, err := Execute(context.Background(), plan, Options{CodexClosed: true}); err == nil {
		t.Fatal("expected expired plan to be rejected")
	}
	plan, err = BuildPlan("/tmp/fixture.zip", testConversations()[:1], map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	plan.Items[0].Data = []byte("changed")
	if _, err := Execute(context.Background(), plan, Options{CodexClosed: true}); err == nil {
		t.Fatal("expected digest mismatch to be rejected")
	}
}

func TestPreflightChecksWritableSpaceAndCodexState(t *testing.T) {
	home, err := codex.NewSyntheticHome(filepath.Join(t.TempDir(), "home-b"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := home.CreateProject("市场调研")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan("/tmp/fixture.zip", testConversations(), map[string]domain.TargetProject{"市场调研": target}, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := Preflight(plan, home.Root, false)
	if err != nil {
		t.Fatal(err)
	}
	if preflight.CanExecute || preflight.CodexClosed || preflight.Message != "请先关闭 Codex" {
		t.Fatalf("unexpected closed-state preflight: %+v", preflight)
	}
	preflight, err = Preflight(plan, home.Root, true)
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.CanExecute || !preflight.Writable || preflight.AvailableBytes < uint64(plan.RequiredBytes) {
		t.Fatalf("unexpected ready preflight: %+v", preflight)
	}
}
