package importexec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

func TestBuildPlanRejectsSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 创建符号链接需要额外权限，集成矩阵单独覆盖")
	}
	home, err := codex.NewSyntheticHome(filepath.Join(t.TempDir(), "home-b"))
	if err != nil {
		t.Fatal(err)
	}
	target, err := home.CreateProject("市场调研")
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(target.Directory, "conversations", "conv-market-brief.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	_, err = BuildPlan("/tmp/fixture.zip", []domain.ConversationSummary{testConversations()[0]}, map[string]domain.TargetProject{"市场调研": target}, ConflictReplace)
	if err == nil {
		t.Fatal("expected symlink target to be rejected")
	}
}
