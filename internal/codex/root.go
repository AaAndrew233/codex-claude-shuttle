package codex

import (
	"errors"
	"os"
	"path/filepath"
)

// DefaultRoot 返回当前用户的 Codex 数据根目录。它只计算路径，不读取任何文件。
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("无法确定当前用户的主目录")
	}
	return filepath.Join(home, ".codex"), nil
}
