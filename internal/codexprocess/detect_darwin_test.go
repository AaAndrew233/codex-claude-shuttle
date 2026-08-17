//go:build darwin

package codexprocess

import (
	"os"
	"strings"
	"testing"
)

func TestRunningFromPSMatchesOnlyDesktopMainProcess(t *testing.T) {
	processes := strings.NewReader(`/Applications/ChatGPT.app/Contents/Frameworks/Codex Framework.framework/Helpers/Codex (Renderer)
/Applications/ChatGPT.app/Contents/Resources/codex app-server --stdio
/Users/test/Applications/ChatGPT.app/Contents/MacOS/ChatGPT
`)
	running, err := runningFromPS(processes)
	if err != nil || !running {
		t.Fatalf("desktop main process was not detected: %v, %v", running, err)
	}
}

func TestPlatformRunningAgainstCurrentDesktop(t *testing.T) {
	if os.Getenv("CODEX_EXPECT_DESKTOP_RUNNING") != "1" {
		t.Skip("set CODEX_EXPECT_DESKTOP_RUNNING=1 for a local integration check")
	}
	running, err := platformRunning()
	if err != nil || !running {
		t.Fatalf("running Codex desktop was not detected: %v, %v", running, err)
	}
}

func TestRunningFromPSIgnoresHelpers(t *testing.T) {
	processes := strings.NewReader(`/Applications/ChatGPT.app/Contents/Frameworks/Codex Framework.framework/Helpers/Codex (Renderer)
/Applications/ChatGPT.app/Contents/Resources/codex app-server --stdio
`)
	running, err := runningFromPS(processes)
	if err != nil || running {
		t.Fatalf("helper process must not be treated as the desktop app: %v, %v", running, err)
	}
}
