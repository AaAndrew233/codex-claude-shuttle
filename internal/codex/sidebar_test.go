package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadVisibleThreadsRejectsInvalidState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".codex-global-state.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadVisibleThreads(root)
	if err == nil || !strings.Contains(err.Error(), "格式暂不兼容") {
		t.Fatalf("expected incompatible state error, got: %v", err)
	}
}

func TestLoadVisibleThreadsRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "state-target.json")
	if err := os.WriteFile(target, []byte(`{"sidebar-project-thread-orders":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".codex-global-state.json")); err != nil {
		t.Fatal(err)
	}
	_, err := loadVisibleThreads(root)
	if err == nil || !strings.Contains(err.Error(), "安全的常规文件") {
		t.Fatalf("expected unsafe state file error, got: %v", err)
	}
}
