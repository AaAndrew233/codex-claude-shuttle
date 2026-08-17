package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

type ImportTarget struct {
	ID        string
	Name      string
	Directory string
}

func ImportTargets(root string) ([]ImportTarget, []domain.CodexImportTarget, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if !filepath.IsAbs(root) {
		return nil, nil, errors.New("Codex 数据根目录必须是绝对路径")
	}
	statePath := filepath.Join(root, ".codex-global-state.json")
	stateInfo, err := os.Lstat(statePath)
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取 Codex 当前项目列表: %w", err)
	}
	if stateInfo.Mode()&os.ModeSymlink != 0 || !stateInfo.Mode().IsRegular() {
		return nil, nil, errors.New("Codex 当前项目列表不是安全的常规文件")
	}
	file, err := os.Open(statePath)
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取 Codex 当前项目列表: %w", err)
	}
	defer file.Close()
	data, err := readStateBounded(file)
	if err != nil {
		return nil, nil, err
	}
	var state codexSidebarState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, nil, errors.New("Codex 当前项目列表格式无效")
	}
	records := make([]ImportTarget, 0, len(state.LocalProjects))
	for key, project := range state.LocalProjects {
		id := strings.TrimSpace(project.ID)
		if id == "" {
			id = key
		}
		name := strings.TrimSpace(project.Name)
		for _, candidate := range project.RootPaths {
			directory := filepath.Clean(strings.TrimSpace(candidate))
			if !filepath.IsAbs(directory) {
				continue
			}
			if info, err := os.Stat(directory); err != nil || !info.IsDir() {
				continue
			}
			if name == "" {
				name = filepath.Base(directory)
			}
			records = append(records, ImportTarget{ID: id, Name: name, Directory: directory})
			break
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Name == records[j].Name {
			return records[i].ID < records[j].ID
		}
		return records[i].Name < records[j].Name
	})
	public := make([]domain.CodexImportTarget, 0, len(records))
	for _, record := range records {
		public = append(public, domain.CodexImportTarget{ID: record.ID, Name: record.Name, Folder: filepath.Base(record.Directory)})
	}
	return records, public, nil
}

func readStateBounded(file *os.File) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxStateBytes {
		return nil, errors.New("Codex 当前项目列表超过安全上限")
	}
	data := make([]byte, info.Size())
	read, err := io.ReadFull(file, data)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if int64(read) != info.Size() {
		return nil, errors.New("Codex 当前项目列表读取不完整")
	}
	return data, nil
}
