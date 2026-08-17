package claude

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	projectsDirectory        = "projects"
	sidebarSessionsDirectory = "local-agent-mode-sessions"
)

func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// DefaultSidebarRoot 返回 Claude Desktop 当前 Code 侧边栏的本地会话目录。
// Claude Desktop 2.1.x 在 macOS 和 Windows 都使用 Claude-3p 用户数据目录。
func DefaultSidebarRoot() (string, error) {
	var userDataRoot string
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		userDataRoot = filepath.Join(home, "Library", "Application Support", "Claude-3p")
	case "windows":
		userDataRoot = strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if userDataRoot == "" {
			configRoot, err := os.UserConfigDir()
			if err != nil {
				return "", err
			}
			userDataRoot = configRoot
		}
		userDataRoot = filepath.Join(userDataRoot, "Claude-3p")
	default:
		configRoot, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		userDataRoot = filepath.Join(configRoot, "Claude-3p")
	}
	return filepath.Join(userDataRoot, sidebarSessionsDirectory), nil
}

func ValidateRoot(root string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if !filepath.IsAbs(root) {
		return "", errors.New("Claude Code 数据根目录必须是绝对路径")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", errors.New("无法访问 Claude Code 数据目录")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("Claude Code 数据根目录必须是安全的现有目录")
	}
	return root, nil
}

func ValidateSidebarRoot(root string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if !filepath.IsAbs(root) {
		return "", errors.New("Claude Code 侧边栏数据目录必须是绝对路径")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", errors.New("无法访问 Claude Code 侧边栏数据目录")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("Claude Code 侧边栏数据目录必须是安全的现有目录")
	}
	return root, nil
}

// EncodeProjectDirectory 与 Claude Code 当前使用的项目目录编码规则一致。
// 该编码不可逆，实际项目路径始终以 JSONL 顶层 cwd 字段为准。
func EncodeProjectDirectory(directory string) string {
	var builder strings.Builder
	builder.Grow(len(directory))
	for _, value := range directory {
		if value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '-' {
			builder.WriteRune(value)
		} else {
			builder.WriteByte('-')
		}
	}
	return builder.String()
}

func ProjectsRoot(root string) string {
	return filepath.Join(root, projectsDirectory)
}
