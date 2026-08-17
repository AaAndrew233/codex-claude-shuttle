//go:build live

package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLiveScanSummary(t *testing.T) {
	root, err := DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	conversationCount := 0
	visibleConversationIDs := make(map[string]struct{})
	for _, project := range result.Projects {
		conversationCount += len(project.Conversations)
		for _, conversation := range project.Conversations {
			visibleConversationIDs[conversation.ID] = struct{}{}
			if isInjectedContext(conversation.Title) || isInjectedContext(conversation.UserMessage) {
				t.Fatalf("injected context visible in project %q", project.Name)
			}
		}
	}
	sidebarThreads, err := loadVisibleThreads(root)
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := collectSessionFiles(root, sidebarThreads)
	if err != nil {
		t.Fatal(err)
	}
	missing := 0
	for _, path := range files {
		threadID := threadIDFromRolloutName(filepath.Base(path))
		if _, ok := visibleConversationIDs[threadID]; !ok {
			missing++
		}
	}
	recentFirst := len(result.Projects) > 0 && result.Projects[0].Name == recentGroupName
	fmt.Fprintf(os.Stdout, "visible groups=%d conversations=%d sidebar_files=%d missing=%d recent_first=%v\n", len(result.Projects), conversationCount, len(files), missing, recentFirst)
	if missing != 0 {
		t.Fatalf("%d sidebar rollout files are missing from the user projection", missing)
	}
}
