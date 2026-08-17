package codexprocess

import (
	"bufio"
	"io"
	"os/exec"
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
	running := false
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if strings.HasSuffix(path, "/ChatGPT.app/Contents/MacOS/ChatGPT") || strings.HasSuffix(path, "/Codex.app/Contents/MacOS/Codex") {
			running = true
		}
	}
	return running, scanner.Err()
}
