package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportTargetsRejectsSymlinkedState(t *testing.T) {
	root := t.TempDir()
	stateTarget := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(stateTarget, []byte(`{"local-projects":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stateTarget, filepath.Join(root, ".codex-global-state.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ImportTargets(root); err == nil || !strings.Contains(err.Error(), "安全的常规文件") {
		t.Fatalf("expected symlinked state rejection, got %v", err)
	}
}
