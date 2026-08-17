package claude

import (
	"bufio"
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
	maxTranscriptFiles      = 200_000
	maxTranscriptBytes      = int64(128 << 20)
	maxSidebarMetadataBytes = int64(1 << 20)
	maxRecordBytes          = 16 << 20
	maxScannedBytes         = int64(8 << 30)
)

type TranscriptSource struct {
	Conversation domain.ConversationSummary
	Path         string
	FileName     string
}

type transcriptLine struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId"`
	CWD         string          `json:"cwd"`
	Timestamp   string          `json:"timestamp"`
	IsSidechain bool            `json:"isSidechain"`
	Message     json.RawMessage `json:"message"`
}

type sidebarSessionMetadata struct {
	SessionID           string   `json:"sessionId"`
	CLISessionID        string   `json:"cliSessionId"`
	Title               string   `json:"title"`
	CWD                 string   `json:"cwd"`
	UserSelectedFolders []string `json:"userSelectedFolders"`
	LastActivityAt      int64    `json:"lastActivityAt"`
	IsArchived          *bool    `json:"isArchived"`
}

type sidebarTranscript struct {
	metadata sidebarSessionMetadata
	path     string
	fileName string
	size     int64
	modTime  time.Time
}

type projectGroup struct {
	key           string
	name          string
	latest        time.Time
	conversations []domain.ConversationSummary
}

func ScanRoot(root string) (domain.ScanResult, error) {
	root, err := ValidateSidebarRoot(root)
	if err != nil {
		return domain.ScanResult{}, err
	}
	transcripts, skipped, scannedBytes, err := discoverSidebarTranscripts(root)
	if err != nil {
		return domain.ScanResult{}, err
	}
	result := domain.ScanResult{Source: "claude-sidebar-read-only", ScannedBytes: scannedBytes, SkippedFiles: skipped, Message: "Claude Code 侧边栏对话扫描完成"}
	groups := make(map[string]*projectGroup)
	for _, transcript := range transcripts {
		conversation, ok, parseErr := parseTranscript(transcript.path, "Claude Code", transcript.modTime)
		if parseErr != nil || !ok || conversation.ID != transcript.metadata.CLISessionID {
			result.SkippedFiles++
			continue
		}
		applySidebarMetadata(&conversation, transcript.metadata)
		key := conversation.SourceDirectory
		if key == "" {
			key = "claude-sidebar"
		}
		name := conversation.SourceProject
		if name == "" {
			name = "Claude Code"
		}
		group := groups[key]
		if group == nil {
			group = &projectGroup{key: key, name: name}
			groups[key] = group
		}
		// 完整来源目录仅供后端导出和映射使用，不进入 WebView 投影。
		conversation.SourceDirectory = ""
		group.conversations = append(group.conversations, conversation)
		updated := parseTimestamp(conversation.UpdatedAt, transcript.modTime)
		if updated.After(group.latest) {
			group.latest = updated
		}
	}

	ordered := make([]*projectGroup, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group.conversations, func(i, j int) bool {
			return parseTimestamp(group.conversations[i].UpdatedAt, time.Time{}).After(parseTimestamp(group.conversations[j].UpdatedAt, time.Time{}))
		})
		ordered = append(ordered, group)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].latest.After(ordered[j].latest) })
	for _, group := range ordered {
		sum := sha256.Sum256([]byte(group.key))
		result.Projects = append(result.Projects, domain.ProjectSummary{ID: "claude-" + hex.EncodeToString(sum[:8]), Name: group.name, ConversationCount: len(group.conversations), Conversations: group.conversations})
	}
	result.Partial = result.SkippedFiles > 0
	if len(result.Projects) == 0 && !result.Partial {
		result.Message = "Claude Code 侧边栏当前没有可迁移对话"
	} else if result.Partial {
		result.Message = fmt.Sprintf("扫描完成，已跳过 %d 个无法安全确认的侧边栏会话", result.SkippedFiles)
	}
	return result, nil
}

func SelectTranscripts(root string, conversationIDs []string) ([]TranscriptSource, error) {
	root, err := ValidateSidebarRoot(root)
	if err != nil {
		return nil, err
	}
	if len(conversationIDs) == 0 {
		return nil, errors.New("至少选择一个对话")
	}
	wanted := make(map[string]bool, len(conversationIDs))
	for _, id := range conversationIDs {
		id = strings.TrimSpace(id)
		if id == "" || wanted[id] {
			return nil, errors.New("对话选择包含空值或重复项")
		}
		wanted[id] = true
	}
	transcripts, _, _, err := discoverSidebarTranscripts(root)
	if err != nil {
		return nil, err
	}
	visible := make(map[string]sidebarTranscript, len(transcripts))
	for _, transcript := range transcripts {
		visible[transcript.metadata.CLISessionID] = transcript
	}
	selected := make([]TranscriptSource, 0, len(conversationIDs))
	for _, requestedID := range conversationIDs {
		id := strings.TrimSpace(requestedID)
		transcript, ok := visible[id]
		if !ok {
			return nil, errors.New("部分所选 Claude Code 对话已归档、删除或不再位于侧边栏，请重新扫描")
		}
		conversation, valid, parseErr := parseTranscript(transcript.path, "Claude Code", transcript.modTime)
		if parseErr != nil || !valid || conversation.ID != id {
			return nil, fmt.Errorf("所选 Claude Code 会话 %s 格式无效", id)
		}
		applySidebarMetadata(&conversation, transcript.metadata)
		selected = append(selected, TranscriptSource{Conversation: conversation, Path: transcript.path, FileName: transcript.fileName})
		delete(wanted, id)
	}
	return selected, nil
}

func discoverSidebarTranscripts(root string) ([]sidebarTranscript, int, int64, error) {
	accountEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("无法读取 Claude Code 侧边栏目录: %w", err)
	}
	candidates := make(map[string][]sidebarTranscript)
	metadataFiles := 0
	skipped := 0
	var scannedBytes int64
	for _, accountEntry := range accountEntries {
		accountPath := filepath.Join(root, accountEntry.Name())
		if !safeDirectory(accountPath) {
			continue
		}
		organizationEntries, readErr := os.ReadDir(accountPath)
		if readErr != nil {
			continue
		}
		for _, organizationEntry := range organizationEntries {
			organizationPath := filepath.Join(accountPath, organizationEntry.Name())
			if !safeDirectory(organizationPath) {
				continue
			}
			entries, entriesErr := os.ReadDir(organizationPath)
			if entriesErr != nil {
				continue
			}
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), "local_") || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				metadataFiles++
				if metadataFiles > maxTranscriptFiles {
					return nil, skipped, scannedBytes, errors.New("Claude Code 侧边栏会话数量超过安全扫描上限")
				}
				metadataPath := filepath.Join(organizationPath, entry.Name())
				metadata, metadataBytes, metadataErr := readSidebarMetadata(metadataPath)
				if metadataErr != nil {
					skipped++
					continue
				}
				if scannedBytes+metadataBytes > maxScannedBytes {
					return nil, skipped, scannedBytes, errors.New("Claude Code 侧边栏会话总大小超过安全扫描上限")
				}
				scannedBytes += metadataBytes
				if *metadata.IsArchived {
					continue
				}
				transcript, transcriptErr := findSidebarTranscript(organizationPath, metadata)
				if transcriptErr != nil {
					skipped++
					continue
				}
				if scannedBytes+transcript.size > maxScannedBytes {
					return nil, skipped, scannedBytes, errors.New("Claude Code 侧边栏会话总大小超过安全扫描上限")
				}
				scannedBytes += transcript.size
				candidates[metadata.CLISessionID] = append(candidates[metadata.CLISessionID], transcript)
			}
		}
	}
	transcripts := make([]sidebarTranscript, 0, len(candidates))
	for _, matches := range candidates {
		if len(matches) != 1 {
			skipped += len(matches)
			continue
		}
		transcripts = append(transcripts, matches[0])
	}
	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].metadata.LastActivityAt > transcripts[j].metadata.LastActivityAt
	})
	return transcripts, skipped, scannedBytes, nil
}

func readSidebarMetadata(path string) (sidebarSessionMetadata, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxSidebarMetadataBytes {
		return sidebarSessionMetadata{}, 0, errors.New("Claude Code 侧边栏元数据不是安全的常规文件")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sidebarSessionMetadata{}, 0, err
	}
	var metadata sidebarSessionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return sidebarSessionMetadata{}, 0, err
	}
	if metadata.IsArchived == nil || !safeIdentifier(metadata.SessionID) || !strings.HasPrefix(metadata.SessionID, "local_") || !safeIdentifier(metadata.CLISessionID) {
		return sidebarSessionMetadata{}, 0, errors.New("Claude Code 侧边栏元数据缺少必要字段")
	}
	if filepath.Base(path) != metadata.SessionID+".json" {
		return sidebarSessionMetadata{}, 0, errors.New("Claude Code 侧边栏元数据与文件名不一致")
	}
	return metadata, info.Size(), nil
}

func findSidebarTranscript(organizationPath string, metadata sidebarSessionMetadata) (sidebarTranscript, error) {
	sessionPath := filepath.Join(organizationPath, metadata.SessionID)
	claudePath := filepath.Join(sessionPath, ".claude")
	projectsPath := filepath.Join(claudePath, projectsDirectory)
	if !safeDirectory(sessionPath) || !safeDirectory(claudePath) || !safeDirectory(projectsPath) {
		return sidebarTranscript{}, errors.New("Claude Code 侧边栏会话目录不可安全读取")
	}
	projectEntries, err := os.ReadDir(projectsPath)
	if err != nil {
		return sidebarTranscript{}, err
	}
	var match sidebarTranscript
	for _, projectEntry := range projectEntries {
		projectPath := filepath.Join(projectsPath, projectEntry.Name())
		if !safeDirectory(projectPath) {
			continue
		}
		path := filepath.Join(projectPath, metadata.CLISessionID+".jsonl")
		info, statErr := os.Lstat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxTranscriptBytes {
			return sidebarTranscript{}, errors.New("Claude Code 侧边栏会话文件不安全")
		}
		if match.path != "" {
			return sidebarTranscript{}, errors.New("Claude Code 侧边栏会话对应多个原始文件")
		}
		match = sidebarTranscript{metadata: metadata, path: path, fileName: filepath.Base(path), size: info.Size(), modTime: info.ModTime()}
	}
	if match.path == "" {
		return sidebarTranscript{}, errors.New("Claude Code 侧边栏会话缺少原始文件")
	}
	return match, nil
}

func applySidebarMetadata(conversation *domain.ConversationSummary, metadata sidebarSessionMetadata) {
	if title := strings.TrimSpace(metadata.Title); title != "" {
		conversation.Title = titleFrom(title)
	}
	for _, folder := range metadata.UserSelectedFolders {
		if clean := cleanAbsolutePath(folder); clean != "" {
			conversation.SourceDirectory = clean
			break
		}
	}
	if conversation.SourceDirectory == "" {
		conversation.SourceDirectory = cleanAbsolutePath(metadata.CWD)
	}
	if conversation.SourceDirectory != "" {
		conversation.SourceProject = filepath.Base(conversation.SourceDirectory)
	}
	if metadata.LastActivityAt > 0 {
		conversation.UpdatedAt = time.UnixMilli(metadata.LastActivityAt).UTC().Format(time.RFC3339Nano)
	}
}

func cleanAbsolutePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) || path == string(filepath.Separator) || path == "." {
		return ""
	}
	return path
}

func safeDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir()
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 160 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func parseTranscript(path, encodedProject string, fallbackTime time.Time) (domain.ConversationSummary, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return domain.ConversationSummary{}, false, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64*1024)
	conversation := domain.ConversationSummary{}
	var firstUser string
	for {
		line, readErr := readLineBounded(reader, maxRecordBytes)
		if len(strings.TrimSpace(string(line))) > 0 {
			var record transcriptLine
			if json.Unmarshal(line, &record) == nil && !record.IsSidechain {
				if conversation.ID == "" && record.SessionID != "" {
					conversation.ID = record.SessionID
				}
				if conversation.SourceDirectory == "" && filepath.IsAbs(record.CWD) {
					conversation.SourceDirectory = filepath.Clean(record.CWD)
				}
				if record.Timestamp != "" {
					conversation.UpdatedAt = record.Timestamp
				}
				text := visibleMessageText(record.Message)
				switch record.Type {
				case "user":
					if text != "" {
						conversation.UserMessage = text
						if firstUser == "" {
							firstUser = text
						}
					}
				case "assistant":
					if text != "" {
						conversation.FinalReply = text
					}
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return domain.ConversationSummary{}, false, readErr
		}
	}
	if conversation.ID == "" {
		conversation.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if firstUser == "" && conversation.FinalReply == "" {
		return domain.ConversationSummary{}, false, nil
	}
	conversation.Title = titleFrom(firstUser)
	if conversation.Title == "" {
		conversation.Title = "Claude Code 对话"
	}
	if conversation.UpdatedAt == "" {
		conversation.UpdatedAt = fallbackTime.UTC().Format(time.RFC3339)
	}
	conversation.SourceProject = filepath.Base(filepath.Clean(conversation.SourceDirectory))
	if conversation.SourceProject == "." || conversation.SourceProject == string(filepath.Separator) || conversation.SourceProject == "" {
		conversation.SourceProject = encodedProject
	}
	return conversation, true, nil
}

func visibleMessageText(raw json.RawMessage) string {
	var message struct {
		Content json.RawMessage `json:"content"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &message) != nil {
		return ""
	}
	var plain string
	if json.Unmarshal(message.Content, &plain) == nil {
		return strings.TrimSpace(plain)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(message.Content, &blocks) != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func readLineBounded(reader *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		fragment, prefix, err := reader.ReadLine()
		if len(line)+len(fragment) > limit {
			return nil, errors.New("Claude Code 会话记录超过安全大小上限")
		}
		line = append(line, fragment...)
		if err != nil {
			return line, err
		}
		if !prefix {
			return line, nil
		}
	}
}

func titleFrom(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	runes := []rune(text)
	if len(runes) > 72 {
		return string(runes[:72]) + "…"
	}
	return text
}

func parseTimestamp(value string, fallback time.Time) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	return fallback
}
