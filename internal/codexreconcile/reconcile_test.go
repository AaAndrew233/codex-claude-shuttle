package codexreconcile

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const helperThreadID = "00000000-0000-4000-8000-000000000202"

func TestReconcileUsesNativeReadAndList(t *testing.T) {
	original := commandContext
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, os.Args[0], "-test.run=TestReconcileHelperProcess")
	}
	t.Cleanup(func() { commandContext = original })
	root := t.TempDir()
	result, err := Reconcile(context.Background(), Options{CodexRoot: root, ThreadIDs: []string{helperThreadID}, Executable: "helper"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Verified) != 1 || result.Verified[0] != helperThreadID {
		t.Fatalf("unexpected reconcile result: %+v", result)
	}
}

func TestReconcileWithInstalledCodexAndTemporaryHome(t *testing.T) {
	if os.Getenv("CODEX_NATIVE_TEST") != "1" {
		t.Skip("set CODEX_NATIVE_TEST=1 for the installed Codex integration check")
	}
	executable, err := FindExecutable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	sessionDirectory := filepath.Join(root, "sessions", "2026", "08", "14")
	if err := os.MkdirAll(sessionDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	rollout := `{"timestamp":"2026-08-14T12:00:00Z","type":"session_meta","payload":{"id":"` + helperThreadID + `","timestamp":"2026-08-14T12:00:00Z","cwd":"` + root + `","originator":"codex_cli_rs","cli_version":"test","source":"cli"}}
{"timestamp":"2026-08-14T12:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"Temporary integration question"}}
{"timestamp":"2026-08-14T12:00:02Z","type":"event_msg","payload":{"type":"agent_message","message":"Temporary integration answer"}}
`
	path := filepath.Join(sessionDirectory, "rollout-2026-08-14T12-00-00-"+helperThreadID+".jsonl")
	if err := os.WriteFile(path, []byte(rollout), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Reconcile(context.Background(), Options{CodexRoot: root, ThreadIDs: []string{helperThreadID}, Executable: executable, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Verified) != 1 || result.Verified[0] != helperThreadID {
		t.Fatalf("installed Codex did not discover the temporary rollout: %+v", result)
	}
}

func TestReconcileHelperProcess(t *testing.T) {
	if os.Getenv("CODEX_HOME") == "" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == 0 {
			continue
		}
		var result any = map[string]any{}
		switch request.Method {
		case "initialize":
			result = map[string]any{"userAgent": "codex-cli/test", "codexHome": os.Getenv("CODEX_HOME")}
		case "thread/list":
			result = map[string]any{"data": []map[string]string{{"id": helperThreadID}}, "nextCursor": nil}
		}
		response, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		_, _ = os.Stdout.Write(append(response, '\n'))
	}
	os.Exit(0)
}
