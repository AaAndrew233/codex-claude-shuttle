//go:build darwin

package claudeprocess

import (
	"bufio"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

func platformRunning() (bool, error) {
	command := exec.Command("/bin/ps", "-axo", "comm=")
	stdout, err := command.StdoutPipe()
	if err != nil {
		return false, err
	}
	if err := command.Start(); err != nil {
		return false, err
	}
	running, scanErr := runningFromPS(stdout)
	waitErr := command.Wait()
	if scanErr != nil {
		return false, scanErr
	}
	if waitErr != nil {
		return false, waitErr
	}
	return running, nil
}

func runningFromPS(reader io.Reader) (bool, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		base := strings.ToLower(filepath.Base(path))
		if base == "claude" || strings.HasSuffix(path, "/Claude.app/Contents/MacOS/Claude") {
			return true, nil
		}
	}
	return false, scanner.Err()
}
