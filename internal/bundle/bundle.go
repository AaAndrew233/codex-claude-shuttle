package bundle

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

const (
	legacyFormatVersion = 1
	formatVersion       = 2
)

// TranscriptSource 是写入迁移包的完整会话来源。Path 只在后端流式读取，永不写入清单。
type TranscriptSource struct {
	Conversation domain.ConversationSummary
	Path         string
	FileName     string
}

type RolloutSource = TranscriptSource

type manifest struct {
	FormatVersion int             `json:"formatVersion"`
	Tool          string          `json:"tool"`
	SourceTool    string          `json:"sourceTool,omitempty"`
	Projects      int             `json:"projects"`
	Conversations int             `json:"conversations"`
	Entries       []manifestEntry `json:"entries"`
}

type manifestEntry struct {
	Path           string `json:"path"`
	Bytes          int64  `json:"bytes"`
	SHA256         string `json:"sha256"`
	ConversationID string `json:"conversationId,omitempty"`
	Kind           string `json:"kind,omitempty"`
}

func Write(path string, conversations []domain.ConversationSummary) (domain.ExportResult, error) {
	if len(conversations) == 0 {
		return domain.ExportResult{}, errors.New("至少选择一个对话")
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return domain.ExportResult{}, errors.New("保存位置必须是绝对路径")
	}
	if filepath.Ext(path) == "" {
		path += ".codex-transfer.zip"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.ExportResult{}, fmt.Errorf("创建保存目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".codex-transfer-*.tmp")
	if err != nil {
		return domain.ExportResult{}, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	zw := zip.NewWriter(tmp)
	entries := make([]manifestEntry, 0, len(conversations))
	seenProjects := map[string]struct{}{}
	for _, conversation := range conversations {
		data, err := json.Marshal(conversation)
		if err != nil {
			_ = zw.Close()
			_ = tmp.Close()
			return domain.ExportResult{}, fmt.Errorf("编码对话失败: %w", err)
		}
		entryPath := "conversations/" + safeName(conversation.ID) + ".json"
		writer, err := zw.Create(entryPath)
		if err != nil {
			_ = zw.Close()
			_ = tmp.Close()
			return domain.ExportResult{}, fmt.Errorf("创建迁移条目失败: %w", err)
		}
		if _, err := writer.Write(data); err != nil {
			_ = zw.Close()
			_ = tmp.Close()
			return domain.ExportResult{}, fmt.Errorf("写入迁移条目失败: %w", err)
		}
		sum := sha256.Sum256(data)
		entries = append(entries, manifestEntry{Path: entryPath, Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
		seenProjects[conversation.SourceProject] = struct{}{}
	}
	m := manifest{FormatVersion: formatVersion, Tool: "codex", Projects: len(seenProjects), Conversations: len(conversations), Entries: entries}
	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return domain.ExportResult{}, fmt.Errorf("编码清单失败: %w", err)
	}
	manifestWriter, err := zw.Create("manifest.json")
	if err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return domain.ExportResult{}, fmt.Errorf("创建清单失败: %w", err)
	}
	if _, err := manifestWriter.Write(manifestData); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return domain.ExportResult{}, fmt.Errorf("写入清单失败: %w", err)
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return domain.ExportResult{}, fmt.Errorf("关闭迁移包失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return domain.ExportResult{}, fmt.Errorf("关闭文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return domain.ExportResult{}, fmt.Errorf("保存迁移包失败: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.ExportResult{}, fmt.Errorf("读取迁移包大小失败: %w", err)
	}
	return domain.ExportResult{Path: path, ProjectCount: len(seenProjects), ConversationCount: len(conversations), Bytes: info.Size(), IntegrityValid: true}, nil
}

// WriteRollouts 将用户选中的完整 rollout 与安全摘要流式写入迁移包。
func WriteRollouts(path string, sources []RolloutSource) (domain.ExportResult, error) {
	return WriteTranscripts(path, "codex", sources)
}

// WriteTranscripts 将所选工具的完整原生 JSONL 与安全摘要流式写入迁移包。
func WriteTranscripts(path, sourceTool string, sources []TranscriptSource) (domain.ExportResult, error) {
	if sourceTool != "codex" && sourceTool != "claude" {
		return domain.ExportResult{}, errors.New("不支持的来源工具")
	}
	if len(sources) == 0 {
		return domain.ExportResult{}, errors.New("至少选择一个对话")
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return domain.ExportResult{}, errors.New("保存位置必须是绝对路径")
	}
	if filepath.Ext(path) == "" {
		path += ".conversation-transfer.zip"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.ExportResult{}, fmt.Errorf("创建保存目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".codex-transfer-*.tmp")
	if err != nil {
		return domain.ExportResult{}, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	zw := zip.NewWriter(tmp)
	entries := make([]manifestEntry, 0, len(sources)*2)
	seenProjects := make(map[string]struct{})
	fail := func(writeErr error) (domain.ExportResult, error) {
		_ = zw.Close()
		_ = tmp.Close()
		return domain.ExportResult{}, writeErr
	}
	for _, source := range sources {
		conversationID := safeName(source.Conversation.ID)
		summaryData, err := json.Marshal(source.Conversation)
		if err != nil {
			return fail(fmt.Errorf("编码对话摘要失败: %w", err))
		}
		summaryPath := "conversations/" + conversationID + ".json"
		summaryWriter, err := zw.Create(summaryPath)
		if err != nil {
			return fail(fmt.Errorf("创建对话摘要失败: %w", err))
		}
		if _, err := summaryWriter.Write(summaryData); err != nil {
			return fail(fmt.Errorf("写入对话摘要失败: %w", err))
		}
		summaryHash := sha256.Sum256(summaryData)
		entries = append(entries, manifestEntry{Path: summaryPath, Bytes: int64(len(summaryData)), SHA256: hex.EncodeToString(summaryHash[:]), ConversationID: source.Conversation.ID, Kind: "summary"})

		sourceInfo, err := os.Lstat(source.Path)
		if err != nil {
			return fail(fmt.Errorf("检查所选对话失败: %w", err))
		}
		if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.Mode().IsRegular() {
			return fail(errors.New("所选对话不是安全的常规文件"))
		}
		rolloutFile, err := os.Open(source.Path)
		if err != nil {
			return fail(fmt.Errorf("读取所选对话失败: %w", err))
		}
		beforeInfo, err := rolloutFile.Stat()
		if err != nil {
			_ = rolloutFile.Close()
			return fail(fmt.Errorf("检查所选对话失败: %w", err))
		}
		if !beforeInfo.Mode().IsRegular() {
			_ = rolloutFile.Close()
			return fail(errors.New("所选对话不是安全的常规文件"))
		}
		rolloutName := safeName(strings.TrimSuffix(source.FileName, filepath.Ext(source.FileName))) + ".jsonl"
		rolloutPath := "rollouts/" + conversationID + "/" + rolloutName
		kind := "rollout"
		if sourceTool == "claude" {
			rolloutPath = "transcripts/claude/" + conversationID + "/" + rolloutName
			kind = "transcript"
		}
		rolloutWriter, err := zw.Create(rolloutPath)
		if err != nil {
			_ = rolloutFile.Close()
			return fail(fmt.Errorf("创建完整对话条目失败: %w", err))
		}
		hasher := sha256.New()
		bytesWritten, copyErr := io.Copy(io.MultiWriter(rolloutWriter, hasher), rolloutFile)
		closeErr := rolloutFile.Close()
		if copyErr != nil {
			return fail(fmt.Errorf("写入完整对话失败: %w", copyErr))
		}
		if closeErr != nil {
			return fail(fmt.Errorf("关闭所选对话失败: %w", closeErr))
		}
		afterInfo, err := os.Lstat(source.Path)
		if err != nil {
			return fail(fmt.Errorf("复核所选对话失败: %w", err))
		}
		if afterInfo.Mode()&os.ModeSymlink != 0 || beforeInfo.Size() != afterInfo.Size() || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) || bytesWritten != beforeInfo.Size() {
			return fail(errors.New("导出期间对话仍在更新，请等待来源工具完成后重新扫描并导出"))
		}
		entries = append(entries, manifestEntry{Path: rolloutPath, Bytes: bytesWritten, SHA256: hex.EncodeToString(hasher.Sum(nil)), ConversationID: source.Conversation.ID, Kind: kind})
		seenProjects[source.Conversation.SourceProject] = struct{}{}
	}
	m := manifest{FormatVersion: formatVersion, Tool: sourceTool, SourceTool: sourceTool, Projects: len(seenProjects), Conversations: len(sources), Entries: entries}
	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fail(fmt.Errorf("编码清单失败: %w", err))
	}
	manifestWriter, err := zw.Create("manifest.json")
	if err != nil {
		return fail(fmt.Errorf("创建清单失败: %w", err))
	}
	if _, err := manifestWriter.Write(manifestData); err != nil {
		return fail(fmt.Errorf("写入清单失败: %w", err))
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return domain.ExportResult{}, fmt.Errorf("关闭迁移包失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return domain.ExportResult{}, fmt.Errorf("关闭文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return domain.ExportResult{}, fmt.Errorf("保存迁移包失败: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.ExportResult{}, fmt.Errorf("读取迁移包大小失败: %w", err)
	}
	return domain.ExportResult{Path: path, ProjectCount: len(seenProjects), ConversationCount: len(sources), Bytes: info.Size(), IntegrityValid: true}, nil
}

func Validate(path string) (domain.BundleValidation, error) {
	archive, err := openValidatedArchive(path)
	if err != nil {
		return domain.BundleValidation{Path: filepath.Clean(path), Message: validationMessage(err)}, err
	}
	defer archive.Close()
	return domain.BundleValidation{Valid: true, Path: archive.Path, SourceTool: archive.Manifest.sourceTool(), ProjectCount: archive.Manifest.Projects, ConversationCount: archive.Manifest.Conversations, Bytes: archive.FileSize, Message: "文件未损坏"}, nil
}

func ReadConversations(path string) ([]domain.ConversationSummary, domain.BundleValidation, error) {
	archive, err := openValidatedArchive(path)
	if err != nil {
		validation := domain.BundleValidation{Path: filepath.Clean(path), Message: validationMessage(err)}
		return nil, validation, err
	}
	defer archive.Close()
	validation := domain.BundleValidation{Valid: true, Path: archive.Path, SourceTool: archive.Manifest.sourceTool(), ProjectCount: archive.Manifest.Projects, ConversationCount: archive.Manifest.Conversations, Bytes: archive.FileSize, Message: "文件未损坏"}
	conversations := make([]domain.ConversationSummary, 0, validation.ConversationCount)
	for _, file := range archive.File {
		if !strings.HasPrefix(file.Name, "conversations/") || !strings.HasSuffix(file.Name, ".json") {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return nil, validation, fmt.Errorf("读取迁移条目失败: %w", err)
		}
		data, err := readCapped(r, maxSummaryBytes, "对话摘要")
		_ = r.Close()
		if err != nil {
			return nil, validation, fmt.Errorf("读取迁移条目失败: %w", err)
		}
		var conversation domain.ConversationSummary
		if err := json.Unmarshal(data, &conversation); err != nil {
			return nil, validation, fmt.Errorf("迁移条目格式无效: %w", err)
		}
		conversations = append(conversations, conversation)
	}
	if len(conversations) != validation.ConversationCount {
		return nil, validation, errors.New("清单中的对话数量与实际条目不一致")
	}
	return conversations, validation, nil
}

func (m manifest) sourceTool() string {
	if m.SourceTool != "" {
		return m.SourceTool
	}
	return m.Tool
}

func safeName(value string) string {
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
