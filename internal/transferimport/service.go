package transferimport

import (
	"bytes"
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
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/claude"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/claudeprocess"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codexprocess"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codexreconcile"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/handoff"
)

const (
	ConflictSkip       = "skip"
	ConflictReplace    = "replace"
	planTTL            = 10 * time.Minute
	maxSessionFiles    = 200_000
	maxConversionBytes = int64(128 << 20)
	maxJournalBytes    = int64(8 << 20)
)

type Service struct {
	mutex     sync.Mutex
	plans     map[string]plan
	running   func(string) (bool, error)
	reconcile func(context.Context, codexreconcile.Options) (codexreconcile.Result, error)
}

type plan struct {
	Token         string
	ExpiresAt     time.Time
	BundlePath    string
	BundleSize    int64
	BundleModTime time.Time
	SourceTool    string
	TargetTool    string
	TargetRoot    string
	Items         []planItem
	RequiredBytes int64
}

type planItem struct {
	Transcript      bundle.ImportTranscript
	SourceKey       string
	TargetDirectory string
	OutputID        string
	Destination     string
	Action          string
	ObservedExists  bool
	ObservedSize    int64
	ObservedModTime time.Time
	ObservedSHA256  string
	OutputSHA256    string
	StagedPath      string
}

type recoveryJournal struct {
	Version    int             `json:"version"`
	TargetRoot string          `json:"targetRoot"`
	TargetTool string          `json:"targetTool"`
	Entries    []recoveryEntry `json:"entries"`
	Completed  bool            `json:"completed,omitempty"`
}

type recoveryEntry struct {
	TargetPath string `json:"targetPath"`
	BackupPath string `json:"backupPath,omitempty"`
	Created    bool   `json:"created"`
	Committed  bool   `json:"committed"`
}

func NewService() *Service {
	return &Service{
		plans: make(map[string]plan),
		running: func(tool string) (bool, error) {
			if tool == "codex" {
				return codexprocess.Running()
			}
			return claudeprocess.Running()
		},
		reconcile: codexreconcile.Reconcile,
	}
}

func (service *Service) Inspect(bundlePath, targetTool, targetRoot string) (domain.TransferImportInspection, error) {
	targetTool, err := validateTool(targetTool)
	if err != nil {
		return domain.TransferImportInspection{}, err
	}
	root, err := validateTargetRoot(targetTool, targetRoot)
	if err != nil {
		return domain.TransferImportInspection{}, err
	}
	transcripts, validation, err := bundle.InspectTranscripts(filepath.Clean(bundlePath))
	if err != nil {
		return domain.TransferImportInspection{}, err
	}
	targets, publicTargets, err := importTargets(targetTool, root)
	if err != nil {
		return domain.TransferImportInspection{}, err
	}
	sources := groupSources(transcripts, targets)
	return domain.TransferImportInspection{
		BundlePath: validation.Path, SourceTool: validation.SourceTool, ProjectCount: len(sources), ConversationCount: len(transcripts),
		Sources: sources, Targets: publicTargets, Message: "迁移包已读取，请确认来源、目标工具和项目位置",
	}, nil
}

func (service *Service) Prepare(request domain.TransferImportRequest) (domain.TransferImportPreflight, error) {
	sourceTool, err := validateTool(request.SourceTool)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	targetTool, err := validateTool(request.TargetTool)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	root, err := validateTargetRoot(targetTool, request.TargetRoot)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	running, err := service.running(targetTool)
	if err != nil {
		return domain.TransferImportPreflight{}, fmt.Errorf("无法检测目标工具运行状态: %w", err)
	}
	if running {
		return domain.TransferImportPreflight{SourceTool: sourceTool, TargetTool: targetTool, ToolRunning: true, Message: "请完全退出目标工具后重新预检"}, nil
	}
	if err := recoverPending(root, targetTool); err != nil {
		return domain.TransferImportPreflight{}, err
	}
	transcripts, validation, err := bundle.InspectTranscripts(filepath.Clean(request.BundlePath))
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	if validation.SourceTool != sourceTool {
		return domain.TransferImportPreflight{}, errors.New("迁移包来源工具与当前选择不一致")
	}
	targets, _, err := importTargets(targetTool, root)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	resolvedMappings, err := resolveMappings(request.Mappings, transcripts, targets, targetTool)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	policy := strings.TrimSpace(request.ConflictPolicy)
	if policy == "" {
		policy = ConflictSkip
	}
	if policy != ConflictSkip && policy != ConflictReplace {
		return domain.TransferImportPreflight{}, errors.New("不支持的冲突处理方式")
	}
	info, err := os.Stat(validation.Path)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	token, err := newToken()
	if err != nil {
		return domain.TransferImportPreflight{}, errors.New("无法创建安全的导入计划")
	}
	prepared := plan{Token: token, ExpiresAt: time.Now().UTC().Add(planTTL), BundlePath: validation.Path, BundleSize: info.Size(), BundleModTime: info.ModTime(), SourceTool: sourceTool, TargetTool: targetTool, TargetRoot: root}
	outputIDs := make(map[string]bool, len(transcripts))
	for _, transcript := range transcripts {
		outputID, idErr := handoff.TargetID(sourceTool, transcript.SessionID, targetTool)
		if idErr != nil {
			return domain.TransferImportPreflight{}, idErr
		}
		outputIDs[outputID] = true
	}
	existing, err := existingSessions(root, targetTool, outputIDs)
	if err != nil {
		return domain.TransferImportPreflight{}, err
	}
	for _, transcript := range transcripts {
		targetDirectory := resolvedMappings[sourceKey(transcript)]
		outputID, _ := handoff.TargetID(sourceTool, transcript.SessionID, targetTool)
		destination, err := destinationPath(root, targetTool, transcript, outputID, targetDirectory)
		if err != nil {
			return domain.TransferImportPreflight{}, err
		}
		if existingPath := existing[outputID]; existingPath != "" {
			destination = existingPath
		}
		item := planItem{Transcript: transcript, SourceKey: sourceKey(transcript), TargetDirectory: targetDirectory, OutputID: outputID, Destination: destination, Action: "create"}
		if err := inspectDestination(&item, policy); err != nil {
			return domain.TransferImportPreflight{}, err
		}
		prepared.Items = append(prepared.Items, item)
		if item.Action != "skip" {
			required := transcript.Bytes
			if sourceTool != targetTool {
				if transcript.Bytes > maxConversionBytes {
					return domain.TransferImportPreflight{}, fmt.Errorf("对话 %s 超过跨工具转换大小上限", transcript.Conversation.Title)
				}
				required *= 2
			}
			prepared.RequiredBytes += required
			if item.Action == "replace" {
				prepared.RequiredBytes += item.ObservedSize
			}
		}
	}

	preflight := domain.TransferImportPreflight{PlanToken: token, SourceTool: sourceTool, TargetTool: targetTool, RequiredBytes: prepared.RequiredBytes, ProjectCount: len(resolvedMappings), ConversationCount: len(prepared.Items)}
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
		preflight.Message = "目标工具数据目录不可写"
		return preflight, nil
	}
	available, err := availableBytes(root)
	if err != nil {
		preflight.Message = "无法确认目标工具数据目录剩余空间"
		return preflight, nil
	}
	preflight.AvailableBytes = available
	if available < uint64(prepared.RequiredBytes) {
		preflight.Message = "目标工具数据目录可用空间不足"
		return preflight, nil
	}
	running, err = service.running(targetTool)
	if err != nil {
		return preflight, err
	}
	preflight.ToolRunning = running
	if running {
		preflight.Message = "请完全退出目标工具后重新预检"
		return preflight, nil
	}
	preflight.CanExecute = true
	preflight.Message = "预检通过，可以开始真实导入"
	service.mutex.Lock()
	service.pruneExpiredLocked()
	service.plans[token] = prepared
	service.mutex.Unlock()
	return preflight, nil
}

func (service *Service) Execute(ctx context.Context, token string) (domain.TransferImportExecutionResult, error) {
	service.mutex.Lock()
	prepared, ok := service.plans[strings.TrimSpace(token)]
	delete(service.plans, strings.TrimSpace(token))
	service.mutex.Unlock()
	if !ok || time.Now().UTC().After(prepared.ExpiresAt) {
		return domain.TransferImportExecutionResult{}, errors.New("导入计划已失效，请重新预检")
	}
	result := domain.TransferImportExecutionResult{SourceTool: prepared.SourceTool, TargetTool: prepared.TargetTool}
	running, err := service.running(prepared.TargetTool)
	if err != nil {
		return result, err
	}
	if running {
		return result, errors.New("目标工具仍在运行，已阻止写入")
	}
	if err := validatePlanInputs(prepared); err != nil {
		return result, err
	}
	result, changedIDs, err := executePlan(ctx, prepared, result)
	if err != nil {
		return result, err
	}
	result.FilesVerified = true
	if len(changedIDs) == 0 {
		result.Message = "没有需要写入的对话，现有内容已保留"
		return result, nil
	}
	result.DiscoveryAttempted = true
	if prepared.TargetTool == "claude" {
		result.Discovered = len(changedIDs)
		result.Message = "对话文件已导入，并满足 Claude Code 本地发现条件"
		return result, nil
	}
	reconciled, reconcileErr := service.reconcile(ctx, codexreconcile.Options{CodexRoot: prepared.TargetRoot, ThreadIDs: changedIDs})
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

func importTargets(tool, root string) ([]codex.ImportTarget, []domain.TransferImportTarget, error) {
	if tool != "codex" {
		return nil, []domain.TransferImportTarget{}, nil
	}
	targets, public, err := codex.ImportTargets(root)
	if err != nil {
		return nil, nil, err
	}
	result := make([]domain.TransferImportTarget, 0, len(public))
	for _, target := range public {
		result = append(result, domain.TransferImportTarget{ID: target.ID, Name: target.Name, Folder: target.Folder})
	}
	return targets, result, nil
}

func groupSources(transcripts []bundle.ImportTranscript, targets []codex.ImportTarget) []domain.TransferImportSource {
	grouped := make(map[string]*domain.TransferImportSource)
	for _, transcript := range transcripts {
		key := sourceKey(transcript)
		source := grouped[key]
		if source == nil {
			name := strings.TrimSpace(transcript.Conversation.SourceProject)
			if name == "" || name == "未命名项目" {
				name = "最近"
			}
			folder := filepath.Base(filepath.Clean(transcript.SourceDirectory))
			if folder == "." || folder == string(filepath.Separator) || folder == "" {
				folder = "未记录来源目录"
			}
			source = &domain.TransferImportSource{Key: key, Name: name, SourceFolder: folder}
			for _, target := range targets {
				if samePath(target.Directory, transcript.SourceDirectory) {
					source.SuggestedTargetID = target.ID
					source.SuggestedTargetName = target.Name
					break
				}
			}
			grouped[key] = source
		}
		source.ConversationCount++
	}
	result := make([]domain.TransferImportSource, 0, len(grouped))
	for _, source := range grouped {
		result = append(result, *source)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func resolveMappings(mappings []domain.TransferProjectMapping, transcripts []bundle.ImportTranscript, targets []codex.ImportTarget, targetTool string) (map[string]string, error) {
	required := make(map[string]bool)
	for _, transcript := range transcripts {
		required[sourceKey(transcript)] = true
	}
	targetByID := make(map[string]codex.ImportTarget, len(targets))
	for _, target := range targets {
		targetByID[target.ID] = target
	}
	resolved := make(map[string]string, len(required))
	encoded := make(map[string]string)
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
		if err := validateProjectDirectory(directory); err != nil {
			return nil, err
		}
		if targetTool == "claude" {
			key := claude.EncodeProjectDirectory(directory)
			if previous := encoded[key]; previous != "" && !samePath(previous, directory) {
				return nil, errors.New("两个目标项目会映射到同一个 Claude Code 存储目录，请选择其他文件夹")
			}
			encoded[key] = directory
		}
		resolved[mapping.SourceKey] = directory
	}
	if len(resolved) != len(required) {
		return nil, errors.New("请为每个来源项目选择目标项目文件夹")
	}
	return resolved, nil
}

func validateTool(tool string) (string, error) {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if tool != "codex" && tool != "claude" {
		return "", errors.New("不支持的迁移工具")
	}
	return tool, nil
}

func validateTargetRoot(tool, root string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if tool == "claude" {
		return claude.ValidateRoot(root)
	}
	if !filepath.IsAbs(root) {
		return "", errors.New("Codex 数据根目录必须是绝对路径")
	}
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("Codex 数据根目录必须是安全的现有目录")
	}
	state, err := os.Lstat(filepath.Join(root, ".codex-global-state.json"))
	if err != nil || state.Mode()&os.ModeSymlink != 0 || !state.Mode().IsRegular() {
		return "", errors.New("所选目录不是可识别的 Codex 数据目录")
	}
	return root, nil
}

func validateProjectDirectory(directory string) error {
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

func inspectDestination(item *planItem, policy string) error {
	info, err := os.Lstat(item.Destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("目标会话 %s 不是安全的常规文件", item.OutputID)
	}
	item.ObservedExists = true
	item.ObservedSize = info.Size()
	item.ObservedModTime = info.ModTime()
	item.ObservedSHA256, err = fileSHA256(item.Destination)
	if err != nil {
		return err
	}
	if policy == ConflictReplace {
		item.Action = "replace"
	} else {
		item.Action = "skip"
	}
	return nil
}

func destinationPath(root, targetTool string, transcript bundle.ImportTranscript, outputID, targetDirectory string) (string, error) {
	var destination, scope string
	if targetTool == "claude" {
		scope = claude.ProjectsRoot(root)
		destination = filepath.Join(scope, claude.EncodeProjectDirectory(targetDirectory), outputID+".jsonl")
	} else {
		date := transcript.SessionTime.UTC()
		fileName := filepath.Base(transcript.FileName)
		if transcript.Tool != "codex" || !strings.HasPrefix(fileName, "rollout-") || !strings.HasSuffix(fileName, ".jsonl") || !strings.Contains(fileName, outputID) {
			fileName = fmt.Sprintf("rollout-%s-%s.jsonl", date.Format("2006-01-02T15-04-05"), outputID)
		}
		scope = filepath.Join(root, "sessions")
		destination = filepath.Join(scope, date.Format("2006"), date.Format("01"), date.Format("02"), fileName)
	}
	if !within(destination, scope) {
		return "", errors.New("目标会话路径超出目标工具数据目录")
	}
	if err := validateExistingDirectoryChain(root, filepath.Dir(destination)); err != nil {
		return "", err
	}
	return destination, nil
}

func validateExistingDirectoryChain(root, destination string) error {
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("目标目录超出工具数据目录")
	}
	current := root
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("目标工具数据目录包含不安全的路径组件")
		}
	}
	return nil
}

func existingSessions(root, tool string, wanted map[string]bool) (map[string]string, error) {
	result := make(map[string]string, len(wanted))
	if tool == "claude" {
		// Claude Code 的会话身份按目标项目目录作用域判断，直接目标路径会在后续检查。
		return result, nil
	}
	scope := filepath.Join(root, "sessions")
	if info, err := os.Lstat(scope); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return nil, err
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("目标会话目录不是安全目录")
	}
	visited := 0
	err := filepath.WalkDir(scope, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		visited++
		if visited > maxSessionFiles {
			return errors.New("目标会话文件数量超过安全扫描上限")
		}
		if entry.IsDir() {
			if tool == "claude" && entry.Name() == "memory" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		id := name
		if tool == "codex" && len(name) >= 36 {
			id = name[len(name)-36:]
		}
		if !wanted[id] {
			return nil
		}
		if previous := result[id]; previous != "" && !samePath(previous, path) {
			return fmt.Errorf("目标数据中重复存在会话 %s，请先在目标工具中处理冲突", id)
		}
		result[id] = path
		return nil
	})
	return result, err
}

func validatePlanInputs(prepared plan) error {
	info, err := os.Stat(prepared.BundlePath)
	if err != nil || info.Size() != prepared.BundleSize || !info.ModTime().Equal(prepared.BundleModTime) {
		return errors.New("迁移包在预检后发生变化，请重新预检")
	}
	transcripts, validation, err := bundle.InspectTranscripts(prepared.BundlePath)
	if err != nil || validation.SourceTool != prepared.SourceTool || len(transcripts) != len(prepared.Items) {
		return errors.New("迁移包在预检后未通过复核，请重新预检")
	}
	byID := make(map[string]bundle.ImportTranscript, len(transcripts))
	for _, transcript := range transcripts {
		byID[transcript.SessionID] = transcript
	}
	for _, item := range prepared.Items {
		current, ok := byID[item.Transcript.SessionID]
		if !ok || current.SHA256 != item.Transcript.SHA256 || current.EntryPath != item.Transcript.EntryPath {
			return errors.New("迁移包会话在预检后发生变化，请重新预检")
		}
		if err := validateProjectDirectory(item.TargetDirectory); err != nil {
			return err
		}
		info, statErr := os.Lstat(item.Destination)
		if item.ObservedExists {
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != item.ObservedSize || !info.ModTime().Equal(item.ObservedModTime) {
				return errors.New("目标会话在预检后发生变化，请重新预检")
			}
			sha, hashErr := fileSHA256(item.Destination)
			if hashErr != nil || sha != item.ObservedSHA256 {
				return errors.New("目标会话内容在预检后发生变化，请重新预检")
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return errors.New("目标会话在预检后出现，请重新预检")
		}
	}
	return nil
}

func executePlan(ctx context.Context, prepared plan, result domain.TransferImportExecutionResult) (domain.TransferImportExecutionResult, []string, error) {
	stageRoot := filepath.Join(prepared.TargetRoot, ".conversation-transfer-staging", prepared.Token)
	if err := validateExistingDirectoryChain(prepared.TargetRoot, filepath.Dir(stageRoot)); err != nil {
		return result, nil, err
	}
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
		outputHash, writeErr := stageOutput(prepared, *item, file)
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil {
			return result, nil, writeErr
		}
		if syncErr != nil {
			return result, nil, syncErr
		}
		if closeErr != nil {
			return result, nil, closeErr
		}
		item.OutputSHA256 = outputHash
	}

	journalRoot := filepath.Join(prepared.TargetRoot, ".conversation-transfer-recovery", prepared.Token)
	if err := validateExistingDirectoryChain(prepared.TargetRoot, filepath.Dir(journalRoot)); err != nil {
		return result, nil, err
	}
	if err := os.MkdirAll(filepath.Join(journalRoot, "backups"), 0o700); err != nil {
		return result, nil, err
	}
	journal := recoveryJournal{Version: 1, TargetRoot: prepared.TargetRoot, TargetTool: prepared.TargetTool}
	changedIDs := make([]string, 0, len(prepared.Items))
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
		if err := validateExistingDirectoryChain(prepared.TargetRoot, filepath.Dir(item.Destination)); err != nil {
			return recoverFailure(result, journalRoot, journal, err)
		}
		if err := os.MkdirAll(filepath.Dir(item.Destination), 0o700); err != nil {
			return recoverFailure(result, journalRoot, journal, err)
		}
		if item.ObservedExists {
			if err := removeRegularFile(item.Destination); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
			if err := os.Rename(item.StagedPath, item.Destination); err != nil {
				return recoverFailure(result, journalRoot, journal, err)
			}
		} else {
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
		changedIDs = append(changedIDs, item.OutputID)
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

func stageOutput(prepared plan, item planItem, destination io.Writer) (string, error) {
	if prepared.SourceTool == "codex" && prepared.TargetTool == "codex" {
		rollout := bundle.ImportRollout{Conversation: item.Transcript.Conversation, EntryPath: item.Transcript.EntryPath, FileName: item.Transcript.FileName, Bytes: item.Transcript.Bytes, SHA256: item.Transcript.SHA256, ThreadID: item.Transcript.SessionID, SourceDirectory: item.Transcript.SourceDirectory, SessionTime: item.Transcript.SessionTime}
		hash, _, err := bundle.CopyMappedRollout(prepared.BundlePath, rollout, item.TargetDirectory, destination)
		return hash, err
	}
	if prepared.SourceTool == "claude" && prepared.TargetTool == "claude" {
		hash, _, err := bundle.CopyMappedClaudeTranscript(prepared.BundlePath, item.Transcript, item.TargetDirectory, destination)
		return hash, err
	}
	data, err := bundle.ReadTranscriptBytes(prepared.BundlePath, item.Transcript, maxConversionBytes)
	if err != nil {
		return "", err
	}
	var session handoff.Session
	if prepared.SourceTool == "codex" {
		session, err = handoff.FromCodex(data)
	} else {
		session, err = handoff.FromClaude(data)
	}
	if err != nil {
		return "", err
	}
	var output []byte
	var id string
	if prepared.TargetTool == "codex" {
		output, id, err = handoff.ToCodex(session, item.TargetDirectory)
	} else {
		output, id, err = handoff.ToClaude(session, item.TargetDirectory)
	}
	if err != nil {
		return "", err
	}
	if id != item.OutputID {
		return "", errors.New("跨工具转换身份与预检计划不一致")
	}
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(destination, hasher), bytes.NewReader(output)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func recoverFailure(result domain.TransferImportExecutionResult, journalRoot string, journal recoveryJournal, cause error) (domain.TransferImportExecutionResult, []string, error) {
	recoveryErr := recoverJournalEntries(journalRoot, journal)
	result.Recovered = recoveryErr == nil
	if recoveryErr != nil {
		return result, nil, fmt.Errorf("导入失败且未能完整恢复: %v; 恢复错误: %w", cause, recoveryErr)
	}
	return result, nil, fmt.Errorf("导入失败，已恢复到写入前状态: %w", cause)
}

func recoverPending(root, targetTool string) error {
	recoveryRoot := filepath.Join(root, ".conversation-transfer-recovery")
	rootInfo, rootErr := os.Lstat(recoveryRoot)
	if errors.Is(rootErr, os.ErrNotExist) {
		return nil
	}
	if rootErr != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return errors.New("导入恢复目录不是安全目录")
	}
	entries, err := os.ReadDir(recoveryRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("导入恢复目录包含不安全的符号链接")
		}
		if !entry.IsDir() {
			continue
		}
		journalRoot := filepath.Join(recoveryRoot, entry.Name())
		journalPath := filepath.Join(journalRoot, "journal.json")
		info, statErr := os.Lstat(journalPath)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxJournalBytes {
			return errors.New("发现无法安全读取的导入恢复记录，请先停止操作")
		}
		file, openErr := os.Open(journalPath)
		if openErr != nil {
			return errors.New("发现无法读取的导入恢复记录，请先停止操作")
		}
		data, err := io.ReadAll(io.LimitReader(file, maxJournalBytes+1))
		_ = file.Close()
		if err != nil {
			return errors.New("发现无法读取的导入恢复记录，请先停止操作")
		}
		var journal recoveryJournal
		if json.Unmarshal(data, &journal) != nil || !samePath(journal.TargetRoot, root) || journal.TargetTool != targetTool {
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
	scope := filepath.Join(journal.TargetRoot, "sessions")
	if journal.TargetTool == "claude" {
		scope = claude.ProjectsRoot(journal.TargetRoot)
	}
	for index := len(journal.Entries) - 1; index >= 0; index-- {
		entry := journal.Entries[index]
		if !within(entry.TargetPath, scope) {
			return errors.New("恢复记录包含越界目标路径")
		}
		if entry.BackupPath != "" {
			if !within(entry.BackupPath, journalRoot) {
				return errors.New("恢复记录包含越界备份路径")
			}
			if _, err := os.Stat(entry.BackupPath); err == nil {
				if err := removeRegularFile(entry.TargetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
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
	file, err := os.CreateTemp(root, ".conversation-transfer-preflight-*")
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

func removeRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("恢复目标不是安全的常规文件")
	}
	return os.Remove(path)
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

func sourceKey(transcript bundle.ImportTranscript) string {
	value := filepath.Clean(transcript.SourceDirectory) + "\x00" + transcript.Conversation.SourceProject
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
