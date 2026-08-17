package codex

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
)

// Probe 只做目录级能力探测，不读取会话正文、数据库或凭据。
// 目录名只能作为候选信号，不能证明当前 Codex 版本兼容。
func Probe(root string) (map[string]any, error) {
	if root == "" {
		return nil, errors.New("未提供 Codex 数据根目录")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("无法访问 Codex 数据根目录: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("Codex 数据根目录不是目录")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("无法读取 Codex 数据根目录: %w", err)
	}
	candidates := make([]string, 0, 3)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		switch entry.Name() {
		case "sessions", "projects", "conversations":
			candidates = append(candidates, entry.Name())
		}
	}
	sort.Strings(candidates)
	return map[string]any{
		"configured":        true,
		"directoryReadable": true,
		"candidateDetected": len(candidates) > 0,
		"candidateNames":    candidates,
		"platform":          runtime.GOOS,
		"message":           "仅完成目录级探测，尚未证明 Codex 格式兼容",
	}, nil
}
