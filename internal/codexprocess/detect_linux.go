package codexprocess

import (
	"errors"
	"os/exec"
)

func platformRunning() (bool, error) {
	path, err := exec.LookPath("pgrep")
	if err != nil {
		return false, nil
	}
	command := exec.Command(path, "-f", `(^|/)(ChatGPT|Codex)( |$)`)
	err = command.Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
