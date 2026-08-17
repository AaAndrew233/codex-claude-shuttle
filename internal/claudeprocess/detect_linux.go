//go:build linux

package claudeprocess

import (
	"errors"
	"os/exec"
)

func platformRunning() (bool, error) {
	path, err := exec.LookPath("pgrep")
	if err != nil {
		return false, nil
	}
	err = exec.Command(path, "-x", "claude").Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
