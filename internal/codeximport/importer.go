package codeximport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/bundle"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codexprocess"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codexreconcile"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

const (
	ConflictSkip    = "skip"
	ConflictReplace = "replace"
	planTTL         = 10 * time.Minute
	maxSessionFiles = 200_000
)

type Service struct {
	mutex     sync.Mutex
	plans     map[string]plan
	running   func() (bool, error)
	reconcile func(context.Context, codexreconcile.Options) (codexreconcile.Result, error)
}

type plan struct {
	Token         string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	BundlePath    string
	BundleSize    int64
	BundleModTime time.Time
	CodexRoot     string
	Items         []planItem
	RequiredBytes int64
}

type planItem struct {
	Rollout         bundle.ImportRollout
	SourceKey       string
	TargetDirectory string
	Destination     string
	Action          string
	ObservedExists  bool
	ObservedSize    int64
	ObservedModTime time.Time
	ObservedSHA256  string
	OutputSHA256    string
	StagedPath      string
}

func NewService() *Service {
	return &Service{plans: make(map[string]plan), running: codexprocess.Running, reconcile: codexreconcile.Reconcile}
}

func (service *Service) Inspect(bundlePath, codexRoot string) (domain.CodexImportInspection, error) {
	root, err := validateCodexRoot(codexRoot)
	if err != nil {
		return domain.CodexImportInspection{}, err
	}
	rollouts, validation, err := bundle.InspectRollouts(filepath.Clean(bundlePath))
	if err != nil {
		return domain.CodexImportInspection{}, err
	}
	targets, publicTargets, err := codex.ImportTargets(root)
	if err != nil {
		return domain.CodexImportInspection{}, err
	}
	sources := groupSources(rollouts, targets)
	return domain.CodexImportInspection{BundlePath: validation.Path, ProjectCount: len(sources), ConversationCount: len(rollouts), Sources: sources, Targets: publicTargets, Message: "迁移包已读取，请确认每个来源项目的目标位置"}, nil
}

func (service *Service) Prepare(request domain.CodexImportRequest) (domain.CodexImportPreflight, error) {
	root, err := validateCodexRoot(request.CodexRoot)
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	running, err := service.running()
	if err != nil {
		return domain.CodexImportPreflight{}, fmt.Errorf("无法检测 Codex 运行状态: %w", err)
	}
	if running {
		return domain.CodexImportPreflight{CodexRunning: true, Message: "请完全退出 Codex 后重新预检"}, nil
	}
	if err := recoverPending(root); err != nil {
		return domain.CodexImportPreflight{}, err
	}
	rollouts, _, err := bundle.InspectRollouts(filepath.Clean(request.BundlePath))
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	targets, _, err := codex.ImportTargets(root)
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	policy := strings.TrimSpace(request.ConflictPolicy)
	if policy == "" {
		policy = ConflictSkip
	}
	if policy != ConflictSkip && policy != ConflictReplace {
		return domain.CodexImportPreflight{}, errors.New("不支持的冲突处理方式")
	}
	resolvedMappings, err := resolveMappings(request.Mappings, rollouts, targets)
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	info, err := os.Stat(request.BundlePath)
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	token, err := newToken()
	if err != nil {
		return domain.CodexImportPreflight{}, errors.New("无法创建安全的导入计划")
	}
	prepared := plan{Token: token, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(planTTL), BundlePath: filepath.Clean(request.BundlePath), BundleSize: info.Size(), BundleModTime: info.ModTime(), CodexRoot: root}
	wantedThreadIDs := make(map[string]bool, len(rollouts))
	for _, rollout := range rollouts {
		wantedThreadIDs[rollout.ThreadID] = true
	}
	existingSessions, err := findExistingSessions(root, wantedThreadIDs)
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	for _, rollout := range rollouts {
		targetDirectory := resolvedMappings[sourceKey(rollout)]
		destination, err := destinationPath(root, rollout)
		if err != nil {
			return domain.CodexImportPreflight{}, err
		}
		if existing := existingSessions[rollout.ThreadID]; existing != "" {
			destination = existing
		}
		item := planItem{Rollout: rollout, SourceKey: sourceKey(rollout), TargetDirectory: targetDirectory, Destination: destination, Action: "create"}
		if existing, err := os.Lstat(destination); err == nil {
			if existing.Mode()&os.ModeSymlink != 0 || !existing.Mode().IsRegular() {
				return domain.CodexImportPreflight{}, fmt.Errorf("目标会话 %s 不是安全的常规文件", rollout.ThreadID)
			}
			item.ObservedExists = true
			item.ObservedSize = existing.Size()
			item.ObservedModTime = existing.ModTime()
			item.ObservedSHA256, err = fileSHA256(destination)
			if err != nil {
				return domain.CodexImportPreflight{}, err
			}
			if policy == ConflictReplace {
				item.Action = "replace"
			} else {
				item.Action = "skip"
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return domain.CodexImportPreflight{}, err
		}
		prepared.Items = append(prepared.Items, item)
		if item.Action != "skip" {
			prepared.RequiredBytes += rollout.Bytes
			if item.Action == "replace" {
				prepared.RequiredBytes += item.ObservedSize
			}
		}
	}

	preflight := domain.CodexImportPreflight{PlanToken: prepared.Token, RequiredBytes: prepared.RequiredBytes, ProjectCount: len(resolvedMappings), ConversationCount: len(prepared.Items)}
	for _, item := range prepared.Items {
		switch item.Action {
		case "create":
			preflight.CreateCount++
		case "replace":
			preflight.ReplaceCount++
		case "skip":
			preflight.SkipCount++
		}
	}
	if err := checkWritable(root); err != nil {
		preflight.Message = "Codex 数据目录不可写"
		return preflight, nil
	}
	available, err := availableBytes(root)
	if err != nil {
		preflight.Message = "无法确认 Codex 数据目录剩余空间"
		return preflight, nil
	}
	preflight.AvailableBytes = available
	if available < uint64(prepared.RequiredBytes) {
		preflight.Message = "Codex 数据目录可用空间不足"
		return preflight, nil
	}
	running, err = service.running()
	if err != nil {
		return preflight, fmt.Errorf("无法检测 Codex 运行状态: %w", err)
	}
	preflight.CodexRunning = running
	if running {
		preflight.Message = "请完全退出 Codex 后重新预检"
		return preflight, nil
	}
	preflight.CanExecute = true
	preflight.Message = "预检通过，可以开始真实导入"
	service.mutex.Lock()
	service.pruneExpiredLocked()
	service.plans[prepared.Token] = prepared
	service.mutex.Unlock()
	return preflight, nil
}

func (service *Service) Execute(ctx context.Context, token string) (domain.CodexImportExecutionResult, error) {
	service.mutex.Lock()
	prepared, ok := service.plans[token]
	delete(service.plans, token)
	service.mutex.Unlock()
	if !ok || time.Now().UTC().After(prepared.ExpiresAt) {
		return domain.CodexImportExecutionResult{}, errors.New("导入计划已失效，请重新预检")
	}
	running, err := service.running()
	if err != nil {
		return domain.CodexImportExecutionResult{}, err
	}
	if running {
		return domain.CodexImportExecutionResult{}, errors.New("Codex 仍在运行，已阻止写入")
	}
	if err := validatePlanInputs(prepared); err != nil {
		return domain.CodexImportExecutionResult{}, err
	}
	result, changedIDs, err := executePlan(ctx, prepared)
	if err != nil {
		return result, err
	}
	result.FilesVerified = true
	if len(changedIDs) == 0 {
		result.Message = "没有需要写入的对话，现有内容已保留"
		return result, nil
	}
	result.DiscoveryAttempted = true
	reconciled, reconcileErr := service.reconcile(ctx, codexreconcile.Options{CodexRoot: prepared.CodexRoot, ThreadIDs: changedIDs})
	result.Discovered = len(reconciled.Verified)
	result.PendingDiscovery = len(changedIDs) - result.Discovered
	result.Warnings = append(result.Warnings, reconciled.Warnings...)
	if reconcileErr != nil {
		result.Warnings = append(result.Warnings, "文件已安全写入，但 Codex 尚未确认发现；请重启 Codex 后检查最近对话")
		result.Message = "对话文件已导入，等待 Codex 重新发现"
		return result, nil
	}
	result.Message = "对话文件已导入，并通过 Codex 原生发现验证"
	return result, nil
}

func groupSources(rollouts []bundle.ImportRollout, targets []codex.ImportTarget) []domain.CodexImportSource {
	grouped := make(map[string]*domain.CodexImportSource)
	for _, rollout := range rollouts {
		key := sourceKey(rollout)
		source := grouped[key]
		if source == nil {
			name := strings.TrimSpace(rollout.Conversation.SourceProject)
			if name == "" || name == "未命名项目" {
				name = "最近"
			}
			folder := filepath.Base(filepath.Clean(rollout.SourceDirectory))
			if folder == "." || folder == string(filepath.Separator) {
				folder = "未记录来源目录"
			}
			source = &domain.CodexImportSource{Key: key, Name: name, SourceFolder: folder}
			for _, target := range targets {
				if samePath(target.Directory, rollout.SourceDirectory) {
					source.SuggestedTargetID = target.ID
					source.SuggestedTargetName = target.Name
					break
				}
			}
			grouped[key] = source
		}
		source.ConversationCount++
	}
	result := make([]domain.CodexImportSource, 0, len(grouped))
	for _, source := range grouped {
		result = append(result, *source)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func resolveMappings(mappings []domain.CodexProjectMapping, rollouts []bundle.ImportRollout, targets []codex.ImportTarget) (map[string]string, error) {
	required := make(map[string]bool)
	for _, rollout := range rollouts {
		required[sourceKey(rollout)] = true
	}
	targetByID := make(map[string]codex.ImportTarget, len(targets))
	for _, target := range targets {
		targetByID[target.ID] = target
	}
	resolved := make(map[string]string, len(required))
	for _, mapping := range mappings {
		if !required[mapping.SourceKey] || resolved[mapping.SourceKey] != "" {
			return nil, errors.New("项目映射包含未知或重复来源")
		}
		directory := ""
		if mapping.TargetID != "" {
			target, ok := targetByID[mapping.TargetID]
			if !ok {
				return nil, errors.New("目标项目已不存在，请重新读取迁移包")
			}
			directory = target.Directory
		} else {
			directory = filepath.Clean(strings.TrimSpace(mapping.TargetDirectory))
		}
		if err := validateTargetDirectory(directory); err != nil {
			return nil, err
		}
		resolved[mapping.SourceKey] = directory
	}
	if len(resolved) != len(required) {
		return nil, errors.New("请为每个来源项目选择目标项目文件夹")
	}
	return resolved, nil
}

func validateTargetDirectory(directory string) error {
	if !filepath.IsAbs(directory) {
		return errors.New("目标项目文件夹必须是绝对路径")
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("无法访问目标项目文件夹: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("目标项目文件夹必须是安全的现有目录")
	}
	return nil
}

func validateCodexRoot(root string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if !filepath.IsAbs(root) {
		return "", errors.New("Codex 数据根目录必须是绝对路径")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("无法访问 Codex 数据根目录: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("Codex 数据根目录必须是安全的现有目录")
	}
	if info, err := os.Stat(filepath.Join(root, ".codex-global-state.json")); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("所选目录不是可识别的 Codex 数据目录")
	}
	return root, nil
}

func destinationPath(root string, rollout bundle.ImportRollout) (string, error) {
	date := rollout.SessionTime.UTC()
	fileName := filepath.Base(rollout.FileName)
	if !strings.HasSuffix(fileName, ".jsonl") || !strings.Contains(fileName, rollout.ThreadID) || !strings.HasPrefix(fileName, "rollout-") {
		fileName = fmt.Sprintf("rollout-%s-%s.jsonl", date.Format("2006-01-02T15-04-05"), rollout.ThreadID)
	}
	destination := filepath.Join(root, "sessions", date.Format("2006"), date.Format("01"), date.Format("02"), fileName)
	if !within(destination, filepath.Join(root, "sessions")) {
		return "", errors.New("目标会话路径超出 Codex sessions 目录")
	}
	return destination, nil
}

func validatePlanInputs(prepared plan) error {
	info, err := os.Stat(prepared.BundlePath)
	if err != nil || info.Size() != prepared.BundleSize || !info.ModTime().Equal(prepared.BundleModTime) {
		return errors.New("迁移包在预检后发生变化，请重新预检")
	}
	if _, _, err := bundle.InspectRollouts(prepared.BundlePath); err != nil {
		return err
	}
	wantedThreadIDs := make(map[string]bool, len(prepared.Items))
	for _, item := range prepared.Items {
		wantedThreadIDs[item.Rollout.ThreadID] = true
	}
	existingSessions, err := findExistingSessions(prepared.CodexRoot, wantedThreadIDs)
	if err != nil {
		return err
	}
	for _, item := range prepared.Items {
		if err := validateTargetDirectory(item.TargetDirectory); err != nil {
			return err
		}
		existingPath := existingSessions[item.Rollout.ThreadID]
		if item.ObservedExists && !samePath(existingPath, item.Destination) {
			return errors.New("目标会话在预检后发生变化，请重新预检")
		}
		if !item.ObservedExists && existingPath != "" {
			return errors.New("目标会话在预检后出现，请重新预检")
		}
		info, err := os.Lstat(item.Destination)
		if item.ObservedExists {
			if err != nil || !info.Mode().IsRegular() || info.Size() != item.ObservedSize || !info.ModTime().Equal(item.ObservedModTime) {
				return errors.New("目标会话在预检后发生变化，请重新预检")
			}
			sha, err := fileSHA256(item.Destination)
			if err != nil || sha != item.ObservedSHA256 {
				return errors.New("目标会话内容在预检后发生变化，请重新预检")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return errors.New("目标会话在预检后出现，请重新预检")
		}
	}
	return nil
}

func executePlan(ctx context.Context, prepared plan) (domain.CodexImportExecutionResult, []string, error) {
	result := domain.CodexImportExecutionResult{}
	stageRoot := filepath.Join(prepared.CodexRoot, ".codex-transfer-staging", prepared.Token)
	if err := os.MkdirAll(stageRoot, 0o700); err != nil {
		return result, nil, err
	}
	defer os.RemoveAll(stageRoot)
	for index := range prepared.Items {
		item := &prepared.Items[index]
		if item.Action == "skip" {
			result.Skipped++
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, nil, err
		}
		item.StagedPath = filepath.Join(stageRoot, fmt.Sprintf("%06d.jsonl", index))
		file, err := os.OpenFile(item.StagedPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return result, nil, err
		}
		sha, _, copyErr := bundle.CopyMappedRollout(prepared.BundlePath, item.Rollout, item.TargetDirectory, file)
		syncErr := file.Sync()
		closeErr := file.Close()
		if copyErr != nil {
			return result, nil, copyErr
		}
		if syncErr != nil {
			return result, nil, syncErr
		}
		if closeErr != nil {
			return result, nil, closeErr
		}
		item.OutputSHA256 = sha
	}

	journalRoot := filepath.Join(prepared.CodexRoot, ".codex-transfer-recovery", prepared.Token)
	if err := os.MkdirAll(filepath.Join(journalRoot, "backups"), 0o700); err != nil {
		return result, nil, err
	}
	journal := recoveryJournal{Version: 1, CodexRoot: prepared.CodexRoot}
	changedIDs := make([]string, 0)
	for index := range prepared.Items {
		item := &prepared.Items[index]
		if item.Action == "skip" {
			continue
		}
		entry := recoveryEntry{TargetPath: item.Destination, Created: !item.ObservedExists}
		if item.ObservedExists {
			entry.BackupPath = filepath.Join(journalRoot, "backups", fmt.Sprintf("%06d.bak", index))
			if sha, err := fileSHA256(item.Destination); err != nil || sha != item.ObservedSHA256 {
				if err == nil {
					err = errors.New("目标会话在提交前发生变化")
				}
				return recoverFailure(result, journalRoot, journal, err)
			}
			if err := copyFile(item.Destination, entry.BackupPath); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
			if sha, err := fileSHA256(entry.BackupPath); err != nil || sha != item.ObservedSHA256 {
				if err == nil {
					err = errors.New("目标会话备份校验失败")
				}
				return recoverFailure(result, journalRoot, journal, err)
			}
		}
		journal.Entries = append(journal.Entries, entry)
		if err := writeJournal(journalRoot, journal); err != nil {
			return recoverFailure(result, journalRoot, journal, err)
		}
		if err := os.MkdirAll(filepath.Dir(item.Destination), 0o700); err != nil {
			return recoverFailure(result, journalRoot, journal, err)
		}
		if item.ObservedExists {
			if err := removeRecoverableTarget(item.Destination); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
			if err := os.Rename(item.StagedPath, item.Destination); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
		} else {
			// Hard-linking is an atomic no-clobber commit on the same filesystem.
			if err := os.Link(item.StagedPath, item.Destination); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
			if err := os.Remove(item.StagedPath); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
		}
		if sha, err := fileSHA256(item.Destination); err != nil || sha != item.OutputSHA256 {
			if err == nil {
				err = errors.New("导入会话读回校验失败")
			}
			return recoverFailure(result, journalRoot, journal, err)
		}
		journal.Entries[len(journal.Entries)-1].Committed = true
		if err := writeJournal(journalRoot, journal); err != nil {
			return recoverFailure(result, journalRoot, journal, err)
		}
		changedIDs = append(changedIDs, item.Rollout.ThreadID)
		if item.Action == "replace" {
			result.Replaced++
		} else {
			result.Written++
		}
	}
	journal.Completed = true
	if err := writeJournal(journalRoot, journal); err != nil {
		return recoverFailure(result, journalRoot, journal, err)
	}
	if err := os.RemoveAll(journalRoot); err != nil {
		result.Warnings = append(result.Warnings, "导入已完成，但恢复临时文件未能自动清理")
	}
	return result, changedIDs, nil
}

type recoveryJournal struct {
	Version   int             `json:"version"`
	CodexRoot string          `json:"codexRoot"`
	Entries   []recoveryEntry `json:"entries"`
	Completed bool            `json:"completed,omitempty"`
}

type recoveryEntry struct {
	TargetPath string `json:"targetPath"`
	BackupPath string `json:"backupPath,omitempty"`
	Created    bool   `json:"created"`
	Committed  bool   `json:"committed"`
}

func recoverFailure(result domain.CodexImportExecutionResult, journalRoot string, journal recoveryJournal, cause error) (domain.CodexImportExecutionResult, []string, error) {
	recoveryErr := recoverJournalEntries(journalRoot, journal)
	result.Recovered = recoveryErr == nil
	if recoveryErr != nil {
		return result, nil, fmt.Errorf("导入失败且未能完整恢复: %v; 恢复错误: %w", cause, recoveryErr)
	}
	return result, nil, fmt.Errorf("导入失败，已恢复到写入前状态: %w", cause)
}

func recoverPending(root string) error {
	recoveryRoot := filepath.Join(root, ".codex-transfer-recovery")
	entries, err := os.ReadDir(recoveryRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		journalRoot := filepath.Join(recoveryRoot, entry.Name())
		data, err := os.ReadFile(filepath.Join(journalRoot, "journal.json"))
		if err != nil {
			return errors.New("发现无法读取的导入恢复记录，请先停止操作")
		}
		var journal recoveryJournal
		if json.Unmarshal(data, &journal) != nil || !samePath(journal.CodexRoot, root) {
			return errors.New("发现无效的导入恢复记录，请先停止操作")
		}
		if journal.Completed {
			if err := os.RemoveAll(journalRoot); err != nil {
				return err
			}
			continue
		}
		if err := recoverJournalEntries(journalRoot, journal); err != nil {
			return err
		}
	}
	return nil
}

func recoverJournalEntries(journalRoot string, journal recoveryJournal) error {
	for index := len(journal.Entries) - 1; index >= 0; index-- {
		entry := journal.Entries[index]
		if !within(entry.TargetPath, filepath.Join(journal.CodexRoot, "sessions")) {
			return errors.New("恢复记录包含越界目标路径")
		}
		if entry.BackupPath != "" {
			if !within(entry.BackupPath, journalRoot) {
				return errors.New("恢复记录包含越界备份路径")
			}
			if _, err := os.Stat(entry.BackupPath); err == nil {
				if err := removeRecoverableTarget(entry.TargetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if err := os.Rename(entry.BackupPath, entry.TargetPath); err != nil {
					return err
				}
			}
		} else if entry.Created {
			if err := os.Remove(entry.TargetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return os.RemoveAll(journalRoot)
}

func findExistingSessions(root string, wanted map[string]bool) (map[string]string, error) {
	result := make(map[string]string, len(wanted))
	sessionsRoot := filepath.Join(root, "sessions")
	if info, err := os.Stat(sessionsRoot); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, errors.New("Codex sessions 路径不是目录")
	}
	visited := 0
	err := filepath.WalkDir(sessionsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		visited++
		if visited > maxSessionFiles {
			return errors.New("Codex 会话文件数量超过安全扫描上限")
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(strings.ToLower(name), ".jsonl") {
			return nil
		}
		withoutExtension := name[:len(name)-len(".jsonl")]
		if len(withoutExtension) < 37 || withoutExtension[len(withoutExtension)-37] != '-' {
			return nil
		}
		threadID := withoutExtension[len(withoutExtension)-36:]
		if !wanted[threadID] {
			return nil
		}
		if existing := result[threadID]; existing != "" && !samePath(existing, path) {
			return fmt.Errorf("目标 Codex 数据中重复存在会话 %s，请先在 Codex 中处理冲突", threadID)
		}
		result[threadID] = path
		return nil
	})
	return result, err
}

func removeRecoverableTarget(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 && !info.Mode().IsRegular() {
		return errors.New("恢复目标不是安全的常规文件")
	}
	return os.Remove(path)
}

func writeJournal(root string, journal recoveryJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(root, ".journal-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(root, "journal.json"))
}

func checkWritable(root string) error {
	file, err := os.CreateTemp(root, ".codex-transfer-preflight-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func sourceKey(rollout bundle.ImportRollout) string {
	value := filepath.Clean(rollout.SourceDirectory) + "\x00" + rollout.Conversation.SourceProject
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func within(path, root string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func newToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func (service *Service) pruneExpiredLocked() {
	now := time.Now().UTC()
	for token, candidate := range service.plans {
		if now.After(candidate.ExpiresAt) {
			delete(service.plans, token)
		}
	}
}
