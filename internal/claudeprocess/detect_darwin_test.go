//go:build darwin

package claudeprocess

import (
	"strings"
	"testing"
)

func TestRunningFromPSMatchesCLIAndDesktopButNotHelpers(t *testing.T) {
	for _, input := range []string{"/usr/local/bin/claude\n", "/Applications/Claude.app/Contents/MacOS/Claude\n"} {
		running, err := runningFromPS(strings.NewReader(input))
		if err != nil || !running {
			t.Fatalf("expected Claude process for %q: %v", input, err)
		}
	}
	running, err := runningFromPS(strings.NewReader("/Applications/Claude.app/Contents/Frameworks/Claude Helper.app/Contents/MacOS/Claude Helper\n"))
	if err != nil || running {
		t.Fatalf("helper must not block import: %v", err)
	}
}
