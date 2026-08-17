package handoff

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCodexToClaudeCarriesOnlyVisibleMessages(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000501"
	data := []byte(`{"timestamp":"2026-08-15T08:00:00Z","type":"session_meta","payload":{"id":"` + id + `","cwd":"/tmp/source"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"真实问题"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"reasoning","summary":"内部推理"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"agent_message","message":"最终回复"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","name":"shell"}}` + "\n")
	session, err := FromCodex(data)
	if err != nil {
		t.Fatal(err)
	}
	output, targetID, err := ToClaude(session, "/tmp/target")
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	if !strings.Contains(text, "真实问题") || !strings.Contains(text, "最终回复") || strings.Contains(text, "内部推理") || strings.Contains(text, "shell") {
		t.Fatalf("unexpected conversion output: %s", text)
	}
	if targetID == id {
		t.Fatal("cross-tool conversion must use a distinct deterministic id")
	}
}

func TestClaudeToCodexDropsThinkingToolsSidechainsAndToolResults(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000502"
	data := []byte(`{"type":"user","sessionId":"` + id + `","cwd":"/tmp/source","message":{"content":"用户问题"}}` + "\n" +
		`{"type":"assistant","sessionId":"` + id + `","cwd":"/tmp/source","message":{"content":[{"type":"thinking","thinking":"秘密"},{"type":"tool_use","name":"Bash"},{"type":"text","text":"助手回复"}]}}` + "\n" +
		`{"type":"user","sessionId":"` + id + `","cwd":"/tmp/source","message":{"content":[{"type":"tool_result","content":"命令输出"}]}}` + "\n" +
		`{"type":"assistant","isSidechain":true,"sessionId":"` + id + `","cwd":"/tmp/source","message":{"content":[{"type":"text","text":"子代理"}]}}` + "\n")
	session, err := FromClaude(data)
	if err != nil {
		t.Fatal(err)
	}
	output, _, err := ToCodex(session, "/tmp/target")
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	if !strings.Contains(text, "用户问题") || !strings.Contains(text, "助手回复") || strings.Contains(text, "秘密") || strings.Contains(text, "Bash") || strings.Contains(text, "命令输出") || strings.Contains(text, "子代理") {
		t.Fatalf("unexpected conversion output: %s", text)
	}
}

func TestRewriteClaudeCWDOnlyChangesTopLevelCWD(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000503"
	original := []byte(`{"type":"user","sessionId":"` + id + `","cwd":"/old","message":{"content":"keep /old"}}` + "\n")
	rewritten, err := RewriteClaudeCWD(original, id, "/new")
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(rewritten), &record); err != nil {
		t.Fatal(err)
	}
	if record["cwd"] != "/new" || !strings.Contains(string(rewritten), "keep /old") {
		t.Fatalf("unexpected rewrite: %s", rewritten)
	}
}

func TestFromClaudeRejectsMixedSessionIDs(t *testing.T) {
	data := []byte(`{"type":"user","sessionId":"00000000-0000-4000-8000-000000000510","message":{"content":"one"}}` + "\n" +
		`{"type":"assistant","sessionId":"00000000-0000-4000-8000-000000000511","message":{"content":[{"type":"text","text":"two"}]}}` + "\n")
	if _, err := FromClaude(data); err == nil || !strings.Contains(err.Error(), "sessionId") {
		t.Fatalf("expected mixed identity rejection, got %v", err)
	}
}
