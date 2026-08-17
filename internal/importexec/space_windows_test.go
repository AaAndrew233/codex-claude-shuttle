//go:build windows

package importexec

import "testing"

func TestAvailableBytesWindows(t *testing.T) {
	available, err := availableBytes(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if available == 0 {
		t.Fatal("expected available disk space")
	}
}

func TestAvailableBytesWindowsRejectsNUL(t *testing.T) {
	if _, err := availableBytes("invalid\x00path"); err == nil {
		t.Fatal("expected invalid path to be rejected")
	}
}
