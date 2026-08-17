package importexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

const (
	ConflictSkip    = "skip"
	ConflictReplace = "replace"
	planTTL         = 10 * time.Minute
)

type PlanItem struct {
	ConversationID string
	TargetPath     string
	Data           []byte
	SourceSHA256   string
	ObservedSHA256 string
	ObservedExists bool
	Action         string
}

type Plan struct {
	ID            string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	BundlePath    string
	Items         []PlanItem
	RequiredBytes int64
	Digest        string
}

type Options struct {
	CodexClosed       bool
	ConflictPolicy    string
	CreateEmptyTarget bool
	BeforeItem        func(index int) error
}

type Result struct {
	Written     int
	Skipped     int
	Replaced    int
	Recovered   bool
	JournalPath string
	Message     string
}

func Preflight(plan Plan, targetRoot string, codexClosed bool) (domain.ImportPreflight, error) {
	targetRoot, err := absoluteDirectory(targetRoot)
	if err != nil {
		return domain.ImportPreflight{}, err
	}
	info, err := os.Stat(targetRoot)
	if err != nil {
		return domain.ImportPreflight{}, fmt.Errorf("无法读取目标工作区: %w", err)
	}
	if !info.IsDir() {
		return domain.ImportPreflight{}, errors.New("目标工作区不是目录")
	}
	writable := false
	tmp, err := os.CreateTemp(targetRoot, ".codex-transfer-preflight-*")
	if err == nil {
		writable = true
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}
	available, spaceErr := availableBytes(targetRoot)
	message := "预检通过，可以开始导入"
	canExecute := writable && codexClosed && (spaceErr == nil && available >= uint64(plan.RequiredBytes))
	if !writable {
		message = "目标工作区不可写"
	} else if !codexClosed {
		message = "请先关闭 Codex"
	} else if spaceErr != nil {
		message = "无法确认目标工作区剩余空间"
	} else if available < uint64(plan.RequiredBytes) {
		message = "目标工作区可用空间不足"
	}
	return domain.ImportPreflight{TargetRoot: targetRoot, RequiredBytes: plan.RequiredBytes, AvailableBytes: available, Writable: writable, CodexClosed: codexClosed, ProjectCount: projectCount(plan), ConversationCount: len(plan.Items), CanExecute: canExecute, Message: message}, nil
}

func projectCount(plan Plan) int {
	projects := map[string]struct{}{}
	for _, item := range plan.Items {
		projects[filepath.Dir(filepath.Dir(item.TargetPath))] = struct{}{}
	}
	return len(projects)
}

type journalEntry struct {
	TargetPath string `json:"targetPath"`
	BackupPath string `json:"backupPath,omitempty"`
	Created    bool   `json:"created"`
	Committed  bool   `json:"committed"`
	Written    bool   `json:"written"`
}

type recoveryJournal struct {
	Version int            `json:"version"`
	Entries []journalEntry `json:"entries"`
}

// BuildPlan 为合成文件格式生成执行计划。目标目录由用户确认后传入。
func BuildPlan(bundlePath string, conversations []domain.ConversationSummary, targets map[string]domain.TargetProject, conflictPolicy string) (Plan, error) {
	if len(conversations) == 0 {
		return Plan{}, errors.New("没有可导入的对话")
	}
	if conflictPolicy != ConflictSkip && conflictPolicy != ConflictReplace {
		return Plan{}, errors.New("不支持的冲突策略")
	}
	items := make([]PlanItem, 0, len(conversations))
	for _, conversation := range conversations {
		target, ok := targets[conversation.SourceProject]
		if !ok {
			return Plan{}, fmt.Errorf("项目 %q 尚未确认目标", conversation.SourceProject)
		}
		targetDirectory, err := absoluteDirectory(target.Directory)
		if err != nil {
			return Plan{}, err
		}
		data, err := json.Marshal(conversation)
		if err != nil {
			return Plan{}, fmt.Errorf("编码对话失败: %w", err)
		}
		name := safeConversationName(conversation.ID)
		targetPath := filepath.Join(targetDirectory, "conversations", name+".json")
		if !isWithin(targetDirectory, targetPath) {
			return Plan{}, errors.New("目标路径超出项目目录")
		}
		if info, err := os.Lstat(targetPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return Plan{}, fmt.Errorf("目标文件是符号链接，拒绝导入: %s", filepath.Base(targetPath))
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Plan{}, fmt.Errorf("检查目标文件失败: %w", err)
		}
		sourceHash := hashBytes(data)
		observedHash, exists, err := fileHash(targetPath)
		if err != nil {
			return Plan{}, fmt.Errorf("读取目标摘要失败: %w", err)
		}
		action := "create"
		if exists && observedHash == sourceHash {
			action = "skip"
		} else if exists && conflictPolicy == ConflictSkip {
			action = "skip"
		} else if exists {
			action = "replace"
		}
		items = append(items, PlanItem{ConversationID: conversation.ID, TargetPath: targetPath, Data: data, SourceSHA256: sourceHash, ObservedSHA256: observedHash, ObservedExists: exists, Action: action})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TargetPath < items[j].TargetPath })
	plan := Plan{ID: fmt.Sprintf("plan-%d", time.Now().UnixNano()), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(planTTL), BundlePath: bundlePath, Items: items}
	for _, item := range items {
		if item.Action != "skip" {
			plan.RequiredBytes += int64(len(item.Data))
		}
	}
	plan.Digest = digestPlan(plan)
	return plan, nil
}

func Execute(ctx context.Context, plan Plan, options Options) (Result, error) {
	if !options.CodexClosed {
		return Result{}, errors.New("导入前必须关闭 Codex")
	}
	if time.Now().UTC().After(plan.ExpiresAt) {
		return Result{}, errors.New("导入计划已过期，请重新生成")
	}
	if digestPlan(plan) != plan.Digest {
		return Result{}, errors.New("导入计划摘要不一致，请重新生成")
	}
	for _, item := range plan.Items {
		currentHash, exists, err := fileHash(item.TargetPath)
		if err != nil {
			return Result{}, fmt.Errorf("导入前检查目标失败: %w", err)
		}
		if exists != item.ObservedExists || exists && currentHash != item.ObservedSHA256 {
			return Result{}, fmt.Errorf("目标内容已变化，需要重新生成导入计划: %s", filepath.Base(item.TargetPath))
		}
	}

	journalDir := filepath.Join(filepath.Dir(filepath.Dir(plan.Items[0].TargetPath)), ".codex-transfer-recovery")
	if err := os.MkdirAll(journalDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("创建恢复目录失败: %w", err)
	}
	journalPath := filepath.Join(journalDir, plan.ID+".json")
	journal := recoveryJournal{Version: 1, Entries: make([]journalEntry, 0, len(plan.Items))}
	result := Result{JournalPath: journalPath}
	for index, item := range plan.Items {
		if item.Action == "skip" {
			result.Skipped++
			continue
		}
		if err := ctx.Err(); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		if options.BeforeItem != nil {
			if err := options.BeforeItem(index); err != nil {
				return recoverWrites(result, journalPath, journal, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(item.TargetPath), 0o755); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		entry := journalEntry{TargetPath: item.TargetPath, Created: !item.ObservedExists}
		if item.ObservedExists {
			backupPath := filepath.Join(journalDir, fmt.Sprintf("%02d-%s.bak", index, safeConversationName(item.ConversationID)))
			if err := copyFile(item.TargetPath, backupPath); err != nil {
				return recoverWrites(result, journalPath, journal, err)
			}
			entry.BackupPath = backupPath
		}
		journal.Entries = append(journal.Entries, entry)
		if err := writeJournal(journalPath, journal); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		if err := atomicWrite(item.TargetPath, item.Data); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		journal.Entries[len(journal.Entries)-1].Written = true
		if err := writeJournal(journalPath, journal); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		if err := verifyFile(item.TargetPath, item.SourceSHA256); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		journal.Entries[len(journal.Entries)-1].Committed = true
		if err := writeJournal(journalPath, journal); err != nil {
			return recoverWrites(result, journalPath, journal, err)
		}
		if item.ObservedExists {
			result.Replaced++
		} else {
			result.Written++
		}
	}
	_ = os.Remove(journalPath)
	result.Message = "文件已写入并完成读回校验"
	return result, nil
}

func recoverWrites(result Result, journalPath string, journal recoveryJournal, cause error) (Result, error) {
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		entry := journal.Entries[i]
		if entry.Created {
			// 创建型条目即使在日志标记提交前失败，也不能把新文件留在目标目录。
			_ = os.Remove(entry.TargetPath)
			continue
		}
		if !entry.Written && !entry.Committed {
			continue
		}
		if entry.BackupPath != "" {
			_ = os.Rename(entry.BackupPath, entry.TargetPath)
		}
	}
	result.Recovered = true
	result.Message = "导入失败，已恢复已执行步骤"
	_ = writeJournal(journalPath, journal)
	return result, fmt.Errorf("%s: %w", result.Message, cause)
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".codex-transfer-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func writeJournal(path string, journal recoveryJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

func fileHash(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return hashBytes(data), true, nil
}

func verifyFile(path, expected string) error {
	actual, exists, err := fileHash(path)
	if err != nil {
		return err
	}
	if !exists || actual != expected {
		return errors.New("读回校验失败")
	}
	return nil
}

func absoluteDirectory(path string) (string, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", errors.New("目标项目必须使用绝对路径")
	}
	return path, nil
}

func isWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func safeConversationName(value string) string {
	value = filepath.Base(value)
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, value)
	if value == "" || value == "." {
		return "conversation"
	}
	return value
}

func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func digestPlan(plan Plan) string {
	h := sha256.New()
	_, _ = io.WriteString(h, plan.ID+"|"+plan.BundlePath)
	for _, item := range plan.Items {
		_, _ = io.WriteString(h, "|"+item.ConversationID+"|"+item.TargetPath+"|"+item.SourceSHA256+"|"+item.ObservedSHA256+"|"+item.Action)
	}
	return hex.EncodeToString(h.Sum(nil))
}
