//go:build live

package claude

import (
	"fmt"
	"os"
	"testing"
)

func TestLiveSidebarScanSummary(t *testing.T) {
	root, err := DefaultSidebarRoot()
	if err != nil {
		t.Fatal(err)
	}
	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	conversationCount := 0
	for _, project := range result.Projects {
		conversationCount += len(project.Conversations)
		for _, conversation := range project.Conversations {
			if conversation.SourceDirectory != "" {
				t.Fatal("Claude sidebar projection exposed a full source directory")
			}
		}
	}
	fmt.Fprintf(os.Stdout, "visible groups=%d conversations=%d skipped=%d partial=%v\n", len(result.Projects), conversationCount, result.SkippedFiles, result.Partial)
}
