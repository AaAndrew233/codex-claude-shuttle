package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanRootShowsOnlySidebarThreads(t *testing.T) {
	root := t.TempDir()
	writeSidebarState(t, root, `{
  "local-projects":{"project-visible":{"id":"project-visible","name":"当前项目"}},
  "sidebar-project-thread-orders":{"project-visible":{"threadIds":["00000000-0000-0000-0000-000000000001"]}},
  "thread-project-assignments":{"00000000-0000-0000-0000-000000000001":{"projectId":"project-visible"}}
}`)
	writeRollout(t, root, "00000000-0000-0000-0000-000000000001", "当前用户问题", "当前助手回答")
	writeRollout(t, root, "00000000-0000-0000-0000-000000000002", "磁盘历史问题", "磁盘历史回答")

	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Projects) != 1 || len(result.Projects[0].Conversations) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	conversation := result.Projects[0].Conversations[0]
	if result.Projects[0].Name != "当前项目" || conversation.ID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("sidebar projection mismatch: %+v", result)
	}
	if conversation.UserMessage != "当前用户问题" || conversation.FinalReply != "当前助手回答" {
		t.Fatalf("unexpected projection: %+v", conversation)
	}
}

func TestScanRootPinsRecentAndSortsProjectsByLatestConversation(t *testing.T) {
	root := t.TempDir()
	recentID := "00000000-0000-0000-0000-000000000011"
	newProjectID := "00000000-0000-0000-0000-000000000012"
	oldProjectID := "00000000-0000-0000-0000-000000000013"
	writeSidebarState(t, root, `{
  "local-projects":{
    "project-new":{"id":"project-new","name":"较新项目"},
    "project-old":{"id":"project-old","name":"较早项目"}
  },
  "sidebar-project-thread-orders":{
    "project-new":{"threadIds":["`+newProjectID+`"]},
    "project-old":{"threadIds":["`+oldProjectID+`"]}
  },
  "projectless-thread-ids":["`+recentID+`"]
}`)
	writeRolloutAt(t, root, recentID, "独立聊天", "独立回答", "2026-08-10T10:00:00Z")
	writeRolloutAt(t, root, newProjectID, "较新问题", "较新回答", "2026-08-13T10:00:00Z")
	writeRolloutAt(t, root, oldProjectID, "较早问题", "较早回答", "2026-08-12T10:00:00Z")

	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"最近", "较新项目", "较早项目"}
	if len(result.Projects) != len(want) {
		t.Fatalf("unexpected project count: %+v", result.Projects)
	}
	for index, name := range want {
		if result.Projects[index].Name != name {
			t.Fatalf("unexpected project order: got %+v want %+v", result.Projects, want)
		}
	}
}

func TestTimestampAfterUsesActualTimeAcrossOffsets(t *testing.T) {
	if timestampAfter("2026-08-13T09:30:00+08:00", "2026-08-13T02:00:00Z") {
		t.Fatal("01:30Z must sort before 02:00Z")
	}
	if !timestampAfter("2026-08-13T10:30:00+08:00", "2026-08-13T02:00:00Z") {
		t.Fatal("02:30Z must sort after 02:00Z")
	}
}

func TestScanRootContinuesAfterOversizedRecord(t *testing.T) {
	root := t.TempDir()
	threadID := "00000000-0000-0000-0000-000000000014"
	writeSidebarState(t, root, `{
  "local-projects":{"project-visible":{"id":"project-visible","name":"当前项目"}},
  "sidebar-project-thread-orders":{"project-visible":{"threadIds":["`+threadID+`"]}}
}`)
	sessions := filepath.Join(root, "sessions", "2026", "08", "13")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"session_meta","payload":{"id":"` + threadID + `","cwd":"/synthetic/current-project"}}
` + strings.Repeat("x", maxRecordBytes+1) + `
{"type":"event_msg","payload":{"type":"user_message","message":"超长记录后的真实问题"}}
{"type":"event_msg","payload":{"type":"agent_message","message":"真实回答"}}
`
	path := filepath.Join(sessions, "rollout-2026-08-13T10-00-00-"+threadID+".jsonl")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Projects) != 1 || len(result.Projects[0].Conversations) != 1 {
		t.Fatalf("oversized record must not hide the conversation: %+v", result)
	}
	if result.Projects[0].Conversations[0].Title != "超长记录后的真实问题" {
		t.Fatalf("unexpected title: %+v", result.Projects[0].Conversations[0])
	}
}

func TestVisibleUserTextFiltersInternalDelegationAndExtractsAttachmentRequest(t *testing.T) {
	if text := visibleUserText("<codex_delegation>\n内部任务\n</codex_delegation>"); text != "" {
		t.Fatalf("delegation context must be hidden: %q", text)
	}
	attachment := "# Files mentioned by the user:\n\n## 示例.png: /private/example.png\n\n## My request:\n修复图片显示问题"
	if text := visibleUserText(attachment); text != "修复图片显示问题" {
		t.Fatalf("unexpected attachment request: %q", text)
	}
	if text := visibleUserText("# Files mentioned by the user:\n\n## 示例.png: /private/example.png"); text != "包含附件的对话" {
		t.Fatalf("attachment-only conversation needs a safe title: %q", text)
	}
}

func TestScanRootKeepsSidebarConversationWithoutVisibleMessages(t *testing.T) {
	root := t.TempDir()
	writeSidebarState(t, root, `{
  "local-projects":{"project-visible":{"id":"project-visible","name":"当前项目"}},
  "sidebar-project-thread-orders":{"project-visible":{"threadIds":["00000000-0000-0000-0000-000000000003"]}}
}`)
	sessions := filepath.Join(root, "sessions", "2026", "08", "13")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"不是可发现对话"}]}}
`
	path := filepath.Join(sessions, "rollout-2026-08-13T10-00-00-00000000-0000-0000-0000-000000000003.jsonl")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Projects) != 1 || len(result.Projects[0].Conversations) != 1 {
		t.Fatalf("sidebar conversation must remain visible: %+v", result)
	}
	conversation := result.Projects[0].Conversations[0]
	if conversation.Title != "未命名对话" || conversation.UserMessage != "" || conversation.FinalReply != "" {
		t.Fatalf("internal response items must not leak: %+v", conversation)
	}
}

func TestScanRootExcludesInjectedCodexContext(t *testing.T) {
	root := t.TempDir()
	threadID := "00000000-0000-0000-0000-000000000004"
	writeSidebarState(t, root, `{
  "local-projects":{"project-visible":{"id":"project-visible","name":"当前项目"}},
  "sidebar-project-thread-orders":{"project-visible":{"threadIds":["`+threadID+`"]}}
}`)
	sessions := filepath.Join(root, "sessions", "2026", "08", "13")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"session_meta","payload":{"id":"` + threadID + `","cwd":"/synthetic/current-project"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions\n\n<INSTRUCTIONS>不要展示</INSTRUCTIONS>"}]}}
{"type":"event_msg","payload":{"type":"user_message","message":"真实用户问题"}}
{"type":"event_msg","payload":{"type":"agent_message","message":"真实助手回答"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context></environment_context>"}]}}
`
	path := filepath.Join(sessions, "rollout-2026-08-13T10-00-00-"+threadID+".jsonl")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	conversation := result.Projects[0].Conversations[0]
	if conversation.Title != "真实用户问题" || conversation.UserMessage != "真实用户问题" {
		t.Fatalf("injected context leaked into conversation: %+v", conversation)
	}
}

func TestScanRootExcludesArchivedSessions(t *testing.T) {
	root := t.TempDir()
	threadID := "00000000-0000-0000-0000-000000000005"
	writeSidebarState(t, root, `{
  "local-projects":{"project-visible":{"id":"project-visible","name":"当前项目"}},
  "sidebar-project-thread-orders":{"project-visible":{"threadIds":["`+threadID+`"]}}
}`)
	archived := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(archived, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"session_meta","payload":{"id":"` + threadID + `","cwd":"/synthetic/current-project"}}
{"type":"event_msg","payload":{"type":"user_message","message":"归档问题"}}
{"type":"event_msg","payload":{"type":"agent_message","message":"归档回答"}}
`
	if err := os.WriteFile(filepath.Join(archived, "rollout-2026-08-13T10-00-00-"+threadID+".jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ScanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Projects) != 0 {
		t.Fatalf("archived conversation should be excluded: %+v", result)
	}
}

func TestScanRootRejectsMissingSidebarState(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := ScanRoot(root)
	if err == nil || !strings.Contains(err.Error(), "当前对话列表") {
		t.Fatalf("expected clear sidebar state error, got: %v", err)
	}
}

func writeSidebarState(t *testing.T, root, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".codex-global-state.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeRollout(t *testing.T, root, threadID, userMessage, assistantMessage string) {
	t.Helper()
	writeRolloutAt(t, root, threadID, userMessage, assistantMessage, "2026-08-13T10:00:00Z")
}

func writeRolloutAt(t *testing.T, root, threadID, userMessage, assistantMessage, timestamp string) {
	t.Helper()
	sessions := filepath.Join(root, "sessions", "2026", "08", "13")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"timestamp":"` + timestamp + `","type":"session_meta","payload":{"id":"` + threadID + `","cwd":"/synthetic/fallback-project"}}
{"timestamp":"` + timestamp + `","type":"event_msg","payload":{"type":"user_message","message":"` + userMessage + `"}}
{"timestamp":"` + timestamp + `","type":"event_msg","payload":{"type":"agent_message","message":"` + assistantMessage + `"}}
`
	path := filepath.Join(sessions, "rollout-2026-08-13T10-00-00-"+threadID+".jsonl")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
