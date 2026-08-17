package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

// SyntheticHome 是隔离测试使用的 Home，不代表真实 Codex 私有格式。
type SyntheticHome struct {
	Root string
}

func NewSyntheticHome(root string) (SyntheticHome, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return SyntheticHome{}, errors.New("合成 Home 必须使用绝对路径")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return SyntheticHome{}, fmt.Errorf("创建合成 Home 失败: %w", err)
	}
	return SyntheticHome{Root: root}, nil
}

func (h SyntheticHome) CreateProject(name string) (domain.TargetProject, error) {
	if name == "" {
		return domain.TargetProject{}, errors.New("项目名称不能为空")
	}
	directory := filepath.Join(h.Root, "projects", name)
	if err := os.MkdirAll(filepath.Join(directory, "conversations"), 0o755); err != nil {
		return domain.TargetProject{}, fmt.Errorf("创建合成项目失败: %w", err)
	}
	return domain.TargetProject{ID: "synthetic-" + name, Name: name, Directory: directory}, nil
}

func (h SyntheticHome) ReadProject(target domain.TargetProject) ([]domain.ConversationSummary, error) {
	directory := filepath.Join(filepath.Clean(target.Directory), "conversations")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("读取合成项目失败: %w", err)
	}
	conversations := make([]domain.ConversationSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("读取合成对话失败: %w", err)
		}
		var conversation domain.ConversationSummary
		if err := json.Unmarshal(data, &conversation); err != nil {
			return nil, fmt.Errorf("合成对话格式无效: %w", err)
		}
		conversations = append(conversations, conversation)
	}
	return conversations, nil
}
