package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/bundle"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/claude"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codeximport"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/importexec"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/importplan"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/transferimport"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	applicationName    = "Codex Claude Shuttle"
	applicationVersion = "0.1.1"
)

// ApplicationInfo 是前端启动时可读取的非敏感应用元数据。
type ApplicationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// App 是 Wails 生命周期与桌面 API 的入口。
type App struct {
	ctx              context.Context
	importer         *codeximport.Service
	transferImporter *transferimport.Service
}

func NewApp() *App {
	return &App{importer: codeximport.NewService(), transferImporter: transferimport.NewService()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) ApplicationInfo() ApplicationInfo {
	return ApplicationInfo{
		Name:    applicationName,
		Version: applicationVersion,
	}
}

// ScanSynthetic 返回安全的合成夹具，供第一条 UI 闭环使用。
func (a *App) ScanSynthetic() domain.ScanResult {
	return codex.SyntheticScan()
}

func (a *App) ScanCodexRoot(root string) (domain.ScanResult, error) {
	resolved, err := resolveCodexRoot(root)
	if err != nil {
		return domain.ScanResult{}, err
	}
	return codex.ScanRoot(resolved)
}

// DefaultCodexRoot 返回默认 Codex 目录，供前端展示和扫描使用。
func (a *App) DefaultCodexRoot() (string, error) {
	return codex.DefaultRoot()
}

func (a *App) ScanClaudeRoot(root string) (domain.ScanResult, error) {
	resolved, err := resolveClaudeSidebarRoot(root)
	if err != nil {
		return domain.ScanResult{}, err
	}
	return claude.ScanRoot(resolved)
}

// DefaultClaudeSidebarRoot 仅用于只读发现 Claude Desktop Code 侧边栏会话。
// 导入仍使用 DefaultClaudeRoot 返回的 ~/.claude，两个边界不能混用。
func (a *App) DefaultClaudeSidebarRoot() (string, error) {
	return claude.DefaultSidebarRoot()
}

func (a *App) DefaultClaudeRoot() (string, error) {
	return claude.DefaultRoot()
}

// ChooseExportFolder 打开系统原生文件夹选择器，返回用户选中的目录。
func (a *App) ChooseExportFolder(defaultDirectory string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未准备好")
	}
	defaultDirectory = strings.TrimSpace(defaultDirectory)
	if defaultDirectory != "" {
		defaultDirectory = filepath.Clean(defaultDirectory)
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: defaultDirectory,
		Title:            "选择迁移包保存文件夹",
	})
}

// ChooseImportFile 打开系统原生文件选择器，返回用户选中的迁移包路径。
func (a *App) ChooseImportFile(defaultDirectory string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未准备好")
	}
	defaultDirectory = strings.TrimSpace(defaultDirectory)
	if defaultDirectory != "" {
		defaultDirectory = filepath.Clean(defaultDirectory)
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: defaultDirectory,
		Title:            "选择 Codex Claude Shuttle 迁移包",
		Filters: []runtime.FileFilter{
			{DisplayName: "Codex Claude Shuttle package (*.zip)", Pattern: "*.zip"},
		},
	})
}

// ChooseImportTargetFolder 让用户为某个来源项目选择目标工作目录。
func (a *App) ChooseImportTargetFolder(defaultDirectory string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("桌面窗口尚未准备好")
	}
	defaultDirectory = strings.TrimSpace(defaultDirectory)
	if defaultDirectory != "" {
		defaultDirectory = filepath.Clean(defaultDirectory)
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: defaultDirectory,
		Title:            "选择目标项目文件夹",
	})
}

// OpenExportFolder 在系统文件管理器中定位已生成的迁移包。
func (a *App) OpenExportFolder(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return errors.New("迁移包路径不能为空")
	}
	if _, err := os.Stat(path); err != nil {
		return errors.New("迁移包文件不存在")
	}

	var command *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		command = exec.Command("open", "-R", path)
	case "windows":
		command = exec.Command("explorer.exe", "/select,", path)
	default:
		command = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := command.Run(); err != nil {
		return errors.New("无法打开迁移包所在文件夹")
	}
	return nil
}

func (a *App) ProbeCodexRoot(root string) (domain.CodexProbe, error) {
	result, err := codex.Probe(filepath.Clean(root))
	if err != nil {
		return domain.CodexProbe{}, err
	}
	names, _ := result["candidateNames"].([]string)
	return domain.CodexProbe{Configured: result["configured"].(bool), DirectoryReadable: result["directoryReadable"].(bool), CandidateDetected: result["candidateDetected"].(bool), CandidateNames: names, Platform: result["platform"].(string), Message: result["message"].(string)}, nil
}

func (a *App) ExportSynthetic(request domain.ExportRequest) (domain.ExportResult, error) {
	if request.Destination == "" {
		return domain.ExportResult{}, errors.New("请选择迁移包保存位置")
	}
	scan := codex.SyntheticScan()
	selected := make([]domain.ConversationSummary, 0, len(request.ConversationIDs))
	wanted := map[string]struct{}{}
	for _, id := range request.ConversationIDs {
		wanted[id] = struct{}{}
	}
	for _, project := range scan.Projects {
		for _, conversation := range project.Conversations {
			if _, ok := wanted[conversation.ID]; ok {
				selected = append(selected, conversation)
			}
		}
	}
	return bundle.Write(filepath.Clean(request.Destination), selected)
}

// ExportCodex 重新核对当前侧边栏，并流式导出用户选中的完整 rollout 与安全摘要。
func (a *App) ExportCodex(root string, request domain.ExportRequest) (domain.ExportResult, error) {
	if request.Destination == "" {
		return domain.ExportResult{}, errors.New("请选择迁移包保存位置")
	}
	resolved, err := resolveCodexRoot(root)
	if err != nil {
		return domain.ExportResult{}, err
	}
	sources, err := codex.SelectRollouts(resolved, request.ConversationIDs)
	if err != nil {
		return domain.ExportResult{}, err
	}
	bundleSources := make([]bundle.RolloutSource, 0, len(sources))
	for _, source := range sources {
		bundleSources = append(bundleSources, bundle.RolloutSource{Conversation: source.Conversation, Path: source.Path, FileName: source.FileName})
	}
	return bundle.WriteRollouts(filepath.Clean(request.Destination), bundleSources)
}

// ExportClaude 仅复核用户选中的 Claude Code 会话文件，不重新执行完整扫描。
func (a *App) ExportClaude(root string, request domain.ExportRequest) (domain.ExportResult, error) {
	if request.Destination == "" {
		return domain.ExportResult{}, errors.New("请选择迁移包保存位置")
	}
	resolved, err := resolveClaudeSidebarRoot(root)
	if err != nil {
		return domain.ExportResult{}, err
	}
	sources, err := claude.SelectTranscripts(resolved, request.ConversationIDs)
	if err != nil {
		return domain.ExportResult{}, err
	}
	bundleSources := make([]bundle.TranscriptSource, 0, len(sources))
	for _, source := range sources {
		bundleSources = append(bundleSources, bundle.TranscriptSource{Conversation: source.Conversation, Path: source.Path, FileName: source.FileName})
	}
	return bundle.WriteTranscripts(filepath.Clean(request.Destination), "claude", bundleSources)
}

func resolveCodexRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return codex.DefaultRoot()
	}
	return filepath.Clean(root), nil
}

func resolveClaudeRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return claude.DefaultRoot()
	}
	return filepath.Clean(root), nil
}

func resolveClaudeSidebarRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return claude.DefaultSidebarRoot()
	}
	return filepath.Clean(root), nil
}

func (a *App) ValidateBundle(path string) (domain.BundleValidation, error) {
	return bundle.Validate(filepath.Clean(path))
}

func (a *App) InspectCodexImport(path, root string) (domain.CodexImportInspection, error) {
	resolved, err := resolveCodexRoot(root)
	if err != nil {
		return domain.CodexImportInspection{}, err
	}
	return a.importer.Inspect(filepath.Clean(path), resolved)
}

func (a *App) PrepareCodexImport(request domain.CodexImportRequest) (domain.CodexImportPreflight, error) {
	resolved, err := resolveCodexRoot(request.CodexRoot)
	if err != nil {
		return domain.CodexImportPreflight{}, err
	}
	request.CodexRoot = resolved
	return a.importer.Prepare(request)
}

func (a *App) ExecuteCodexImport(planToken string) (domain.CodexImportExecutionResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.importer.Execute(ctx, strings.TrimSpace(planToken))
}

func (a *App) InspectTransferImport(path, targetTool, targetRoot string) (domain.TransferImportInspection, error) {
	if strings.TrimSpace(targetTool) == "" {
		validation, err := bundle.Validate(filepath.Clean(path))
		if err != nil {
			return domain.TransferImportInspection{}, err
		}
		targetTool = validation.SourceTool
	}
	if strings.TrimSpace(targetRoot) == "" {
		var err error
		if strings.EqualFold(strings.TrimSpace(targetTool), "claude") {
			targetRoot, err = claude.DefaultRoot()
		} else {
			targetRoot, err = codex.DefaultRoot()
		}
		if err != nil {
			return domain.TransferImportInspection{}, err
		}
	}
	return a.transferImporter.Inspect(filepath.Clean(path), targetTool, filepath.Clean(targetRoot))
}

func (a *App) PrepareTransferImport(request domain.TransferImportRequest) (domain.TransferImportPreflight, error) {
	return a.transferImporter.Prepare(request)
}

func (a *App) ExecuteTransferImport(planToken string) (domain.TransferImportExecutionResult, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.transferImporter.Execute(ctx, strings.TrimSpace(planToken))
}

func (a *App) PlanSyntheticImport(path string, targets []domain.TargetProject) (domain.ImportPlan, error) {
	conversations, _, err := bundle.ReadConversations(filepath.Clean(path))
	if err != nil {
		return domain.ImportPlan{}, err
	}
	return importplan.Build(filepath.Clean(path), conversations, targets), nil
}

func (a *App) PreflightSyntheticImport(path, targetRoot string, codexClosed bool) (domain.ImportPreflight, error) {
	conversations, _, err := bundle.ReadConversations(filepath.Clean(path))
	if err != nil {
		return domain.ImportPreflight{}, err
	}
	target := domain.TargetProject{ID: "synthetic-target-market", Name: "市场调研", Directory: filepath.Join(filepath.Clean(targetRoot), "市场调研")}
	plan, err := importexec.BuildPlan(filepath.Clean(path), conversations, map[string]domain.TargetProject{"市场调研": target}, importexec.ConflictSkip)
	if err != nil {
		return domain.ImportPreflight{}, err
	}
	return importexec.Preflight(plan, filepath.Clean(targetRoot), codexClosed)
}

func (a *App) ExecuteSyntheticImport(path, targetRoot string, codexClosed bool) (domain.ImportExecutionResult, error) {
	conversations, _, err := bundle.ReadConversations(filepath.Clean(path))
	if err != nil {
		return domain.ImportExecutionResult{}, err
	}
	target := domain.TargetProject{ID: "synthetic-target-market", Name: "市场调研", Directory: filepath.Join(filepath.Clean(targetRoot), "市场调研")}
	plan, err := importexec.BuildPlan(filepath.Clean(path), conversations, map[string]domain.TargetProject{"市场调研": target}, importexec.ConflictSkip)
	if err != nil {
		return domain.ImportExecutionResult{}, err
	}
	preflight, err := importexec.Preflight(plan, filepath.Clean(targetRoot), codexClosed)
	if err != nil {
		return domain.ImportExecutionResult{}, err
	}
	if !preflight.CanExecute {
		return domain.ImportExecutionResult{}, errors.New(preflight.Message)
	}
	result, err := importexec.Execute(context.Background(), plan, importexec.Options{CodexClosed: codexClosed})
	return domain.ImportExecutionResult{Written: result.Written, Skipped: result.Skipped, Replaced: result.Replaced, Recovered: result.Recovered, Message: result.Message}, err
}
