package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProbeOnlyReportsCandidateDirectoryNames(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "auth.json"), []byte("synthetic-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Probe(root)
	if err != nil {
		t.Fatal(err)
	}
	if result["candidateDetected"] != true {
		t.Fatalf("unexpected probe: %+v", result)
	}
	if result["candidateNames"].([]string)[0] != "sessions" {
		t.Fatalf("unexpected candidate names: %+v", result)
	}
}
