package bundle

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"archive/zip"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

const (
	maxManifestBytes      = int64(1 << 20)
	maxSummaryBytes       = int64(16 << 20)
	maxSessionMetaBytes   = 4 << 20
	maxJSONLRecordBytes   = 16 << 20
	maxRolloutBytes       = int64(4 << 30)
	maxBundleUncompressed = int64(64 << 30)
	maxBundleEntries      = 200_000
)

var threadIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type validatedArchive struct {
	*zip.ReadCloser
	Path     string
	Manifest manifest
	Files    map[string]*zip.File
	FileSize int64
}

type ImportRollout struct {
	Conversation    domain.ConversationSummary
	EntryPath       string
	FileName        string
	Bytes           int64
	SHA256          string
	ThreadID        string
	SourceDirectory string
	SessionTime     time.Time
}

type ImportTranscript struct {
	Tool            string
	Conversation    domain.ConversationSummary
	EntryPath       string
	FileName        string
	Bytes           int64
	SHA256          string
	SessionID       string
	SourceDirectory string
	SessionTime     time.Time
}

func openValidatedArchive(path string) (*validatedArchive, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return nil, errors.New("迁移包路径必须是绝对路径")
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("打开迁移包失败: %w", err)
	}
	fail := func(err error) (*validatedArchive, error) {
		_ = reader.Close()
		return nil, err
	}
	if len(reader.File) > maxBundleEntries {
		return fail(fmt.Errorf("迁移包文件数量超过安全上限 %d", maxBundleEntries))
	}

	files := make(map[string]*zip.File, len(reader.File))
	var manifestFile *zip.File
	for _, file := range reader.File {
		if err := validateArchivePath(file.Name); err != nil {
			return fail(err)
		}
		if _, exists := files[file.Name]; exists {
			return fail(fmt.Errorf("迁移包包含重复条目 %q", file.Name))
		}
		files[file.Name] = file
		if file.Name == "manifest.json" {
			manifestFile = file
		}
	}
	if manifestFile == nil {
		return fail(errors.New("迁移包缺少 manifest.json"))
	}
	manifestData, err := readZipEntryCapped(manifestFile, maxManifestBytes, "迁移包清单")
	if err != nil {
		return fail(err)
	}
	var m manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		return fail(errors.New("迁移包清单格式无效"))
	}
	sourceTool := m.sourceTool()
	if (m.FormatVersion != legacyFormatVersion && m.FormatVersion != formatVersion) || (sourceTool != "codex" && sourceTool != "claude") {
		return fail(errors.New("不是受支持的对话迁移包"))
	}
	if m.SourceTool != "" && m.Tool != "" && m.SourceTool != m.Tool {
		return fail(errors.New("迁移包来源工具声明不一致"))
	}
	if m.FormatVersion == legacyFormatVersion && sourceTool != "codex" {
		return fail(errors.New("旧版迁移包只支持 Codex"))
	}
	if m.Projects < 0 || m.Conversations < 0 {
		return fail(errors.New("迁移包清单计数无效"))
	}

	declared := make(map[string]manifestEntry, len(m.Entries))
	var total int64
	summaryCount := 0
	transcriptCount := 0
	for _, entry := range m.Entries {
		if err := validateArchivePath(entry.Path); err != nil {
			return fail(err)
		}
		if _, exists := declared[entry.Path]; exists {
			return fail(fmt.Errorf("清单重复声明条目 %q", entry.Path))
		}
		file, ok := files[entry.Path]
		if !ok || file.FileInfo().IsDir() {
			return fail(fmt.Errorf("迁移包缺少清单条目 %q", entry.Path))
		}
		limit := maxSummaryBytes
		if entry.Kind == "rollout" || entry.Kind == "transcript" {
			limit = maxRolloutBytes
			transcriptCount++
		} else if strings.HasPrefix(entry.Path, "conversations/") {
			summaryCount++
		}
		if entry.Bytes < 0 || entry.Bytes > limit {
			return fail(fmt.Errorf("迁移包条目 %q 超过安全上限", entry.Path))
		}
		if file.UncompressedSize64 != uint64(entry.Bytes) {
			return fail(fmt.Errorf("迁移包条目 %q 大小与清单不一致", entry.Path))
		}
		if len(entry.SHA256) != sha256.Size*2 {
			return fail(fmt.Errorf("迁移包条目 %q 摘要格式无效", entry.Path))
		}
		if total > maxBundleUncompressed-entry.Bytes {
			return fail(errors.New("迁移包展开后总大小超过安全上限"))
		}
		total += entry.Bytes
		if err := verifyZipEntry(file, entry); err != nil {
			return fail(err)
		}
		declared[entry.Path] = entry
	}
	for _, file := range reader.File {
		if file.Name == "manifest.json" || file.FileInfo().IsDir() {
			continue
		}
		if _, ok := declared[file.Name]; !ok {
			return fail(fmt.Errorf("迁移包包含未在清单声明的条目 %q", file.Name))
		}
	}
	if summaryCount != m.Conversations {
		return fail(errors.New("清单中的对话数量与摘要条目不一致"))
	}
	if transcriptCount != 0 && transcriptCount != m.Conversations {
		return fail(errors.New("清单中的对话数量与完整会话条目不一致"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return fail(err)
	}
	return &validatedArchive{ReadCloser: reader, Path: path, Manifest: m, Files: files, FileSize: info.Size()}, nil
}

func InspectRollouts(path string) ([]ImportRollout, domain.BundleValidation, error) {
	archive, err := openValidatedArchive(path)
	if err != nil {
		validation := domain.BundleValidation{Path: filepath.Clean(path), Message: validationMessage(err)}
		return nil, validation, err
	}
	defer archive.Close()
	validation := domain.BundleValidation{Valid: true, Path: archive.Path, SourceTool: archive.Manifest.sourceTool(), ProjectCount: archive.Manifest.Projects, ConversationCount: archive.Manifest.Conversations, Bytes: archive.FileSize, Message: "文件未损坏"}
	if validation.SourceTool != "codex" {
		return nil, validation, errors.New("迁移包不是 Codex 原生会话包")
	}

	summaries := make(map[string]domain.ConversationSummary, archive.Manifest.Conversations)
	for _, entry := range archive.Manifest.Entries {
		if entry.Kind == "rollout" || !strings.HasPrefix(entry.Path, "conversations/") {
			continue
		}
		data, err := readZipEntryCapped(archive.Files[entry.Path], maxSummaryBytes, "对话摘要")
		if err != nil {
			return nil, validation, err
		}
		var summary domain.ConversationSummary
		if err := json.Unmarshal(data, &summary); err != nil || !threadIDPattern.MatchString(summary.ID) {
			return nil, validation, fmt.Errorf("迁移包中的对话摘要 %q 无效", entry.Path)
		}
		if entry.ConversationID != "" && entry.ConversationID != summary.ID {
			return nil, validation, fmt.Errorf("迁移包中的对话身份不一致: %s", summary.ID)
		}
		if _, exists := summaries[summary.ID]; exists {
			return nil, validation, fmt.Errorf("迁移包重复包含对话 %s", summary.ID)
		}
		summaries[summary.ID] = summary
	}

	rollouts := make([]ImportRollout, 0, archive.Manifest.Conversations)
	seen := make(map[string]bool, archive.Manifest.Conversations)
	for _, entry := range archive.Manifest.Entries {
		if entry.Kind != "rollout" {
			continue
		}
		file := archive.Files[entry.Path]
		meta, err := readSessionMeta(file)
		if err != nil {
			return nil, validation, fmt.Errorf("读取完整会话 %q 失败: %w", entry.Path, err)
		}
		if !threadIDPattern.MatchString(meta.ID) || entry.ConversationID != meta.ID {
			return nil, validation, fmt.Errorf("完整会话 %q 的线程身份无效", entry.Path)
		}
		summary, ok := summaries[meta.ID]
		if !ok || seen[meta.ID] {
			return nil, validation, fmt.Errorf("完整会话 %q 没有唯一匹配的摘要", entry.Path)
		}
		seen[meta.ID] = true
		sessionTime := parseSessionTime(meta.Timestamp, summary.UpdatedAt)
		rollouts = append(rollouts, ImportRollout{Conversation: summary, EntryPath: entry.Path, FileName: filepath.Base(entry.Path), Bytes: entry.Bytes, SHA256: entry.SHA256, ThreadID: meta.ID, SourceDirectory: meta.CWD, SessionTime: sessionTime})
	}
	if len(rollouts) != archive.Manifest.Conversations {
		return nil, validation, errors.New("迁移包不包含可恢复的完整 Codex 会话；旧版摘要包不能执行真实导入")
	}
	return rollouts, validation, nil
}

// InspectTranscripts 返回经过完整性校验的原生会话元数据，不读取消息正文到内存。
func InspectTranscripts(path string) ([]ImportTranscript, domain.BundleValidation, error) {
	archive, err := openValidatedArchive(path)
	if err != nil {
		validation := domain.BundleValidation{Path: filepath.Clean(path), Message: validationMessage(err)}
		return nil, validation, err
	}
	defer archive.Close()
	tool := archive.Manifest.sourceTool()
	validation := domain.BundleValidation{Valid: true, Path: archive.Path, SourceTool: tool, ProjectCount: archive.Manifest.Projects, ConversationCount: archive.Manifest.Conversations, Bytes: archive.FileSize, Message: "文件未损坏"}

	summaries := make(map[string]domain.ConversationSummary, archive.Manifest.Conversations)
	for _, entry := range archive.Manifest.Entries {
		if entry.Kind != "summary" && !strings.HasPrefix(entry.Path, "conversations/") {
			continue
		}
		if !strings.HasPrefix(entry.Path, "conversations/") {
			continue
		}
		data, readErr := readZipEntryCapped(archive.Files[entry.Path], maxSummaryBytes, "对话摘要")
		if readErr != nil {
			return nil, validation, readErr
		}
		var summary domain.ConversationSummary
		if json.Unmarshal(data, &summary) != nil || !threadIDPattern.MatchString(summary.ID) {
			return nil, validation, fmt.Errorf("迁移包中的对话摘要 %q 无效", entry.Path)
		}
		if entry.ConversationID != "" && entry.ConversationID != summary.ID {
			return nil, validation, errors.New("迁移包中的对话身份不一致")
		}
		if _, exists := summaries[summary.ID]; exists {
			return nil, validation, fmt.Errorf("迁移包重复包含对话 %s", summary.ID)
		}
		summaries[summary.ID] = summary
	}

	result := make([]ImportTranscript, 0, archive.Manifest.Conversations)
	seen := make(map[string]bool, archive.Manifest.Conversations)
	for _, entry := range archive.Manifest.Entries {
		if entry.Kind != "rollout" && entry.Kind != "transcript" {
			continue
		}
		var id, cwd, timestamp string
		if tool == "codex" {
			meta, metaErr := readSessionMeta(archive.Files[entry.Path])
			if metaErr != nil {
				return nil, validation, fmt.Errorf("读取 Codex 完整会话失败: %w", metaErr)
			}
			id, cwd, timestamp = meta.ID, meta.CWD, meta.Timestamp
		} else {
			metaID, metaCWD, metaTimestamp, metaErr := readClaudeTranscriptMeta(archive.Files[entry.Path])
			if metaErr != nil {
				return nil, validation, fmt.Errorf("读取 Claude Code 完整会话失败: %w", metaErr)
			}
			id, cwd, timestamp = metaID, metaCWD, metaTimestamp
		}
		if !threadIDPattern.MatchString(id) || entry.ConversationID != id || seen[id] {
			return nil, validation, fmt.Errorf("完整会话 %q 的身份无效或重复", entry.Path)
		}
		summary, ok := summaries[id]
		if !ok {
			return nil, validation, fmt.Errorf("完整会话 %q 没有匹配摘要", entry.Path)
		}
		seen[id] = true
		result = append(result, ImportTranscript{Tool: tool, Conversation: summary, EntryPath: entry.Path, FileName: filepath.Base(entry.Path), Bytes: entry.Bytes, SHA256: entry.SHA256, SessionID: id, SourceDirectory: cwd, SessionTime: parseSessionTime(timestamp, summary.UpdatedAt)})
	}
	if len(result) != archive.Manifest.Conversations {
		return nil, validation, errors.New("迁移包不包含可恢复的完整原生会话")
	}
	return result, validation, nil
}

func ReadTranscriptBytes(bundlePath string, transcript ImportTranscript, limit int64) ([]byte, error) {
	if limit <= 0 || limit > maxRolloutBytes {
		limit = maxRolloutBytes
	}
	reader, err := zip.OpenReader(filepath.Clean(bundlePath))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != transcript.EntryPath {
			continue
		}
		if file.UncompressedSize64 != uint64(transcript.Bytes) || transcript.Bytes > limit {
			return nil, errors.New("迁移包条目在执行前发生变化或超过转换上限")
		}
		data, readErr := readZipEntryCapped(file, limit, "完整会话")
		if readErr != nil {
			return nil, readErr
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != transcript.SHA256 {
			return nil, errors.New("迁移包条目在执行前未通过完整性复核")
		}
		return data, nil
	}
	return nil, errors.New("迁移包缺少完整会话条目")
}

func readClaudeTranscriptMeta(file *zip.File) (string, string, string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", "", "", err
	}
	defer reader.Close()
	scanner := bufio.NewScanner(io.LimitReader(reader, maxSessionMetaBytes+1))
	scanner.Buffer(make([]byte, 64*1024), maxSessionMetaBytes)
	for scanner.Scan() {
		var record struct {
			SessionID string `json:"sessionId"`
			CWD       string `json:"cwd"`
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) == nil && record.SessionID != "" {
			return strings.TrimSpace(record.SessionID), strings.TrimSpace(record.CWD), strings.TrimSpace(record.Timestamp), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", "", err
	}
	return "", "", "", errors.New("未找到 Claude Code sessionId")
}

func CopyMappedRollout(bundlePath string, rollout ImportRollout, targetCWD string, destination io.Writer) (string, int64, error) {
	reader, err := zip.OpenReader(filepath.Clean(bundlePath))
	if err != nil {
		return "", 0, fmt.Errorf("重新打开迁移包失败: %w", err)
	}
	defer reader.Close()
	var file *zip.File
	for _, candidate := range reader.File {
		if candidate.Name == rollout.EntryPath {
			file = candidate
			break
		}
	}
	if file == nil || file.UncompressedSize64 != uint64(rollout.Bytes) {
		return "", 0, errors.New("迁移包在执行前发生变化")
	}
	raw, err := file.Open()
	if err != nil {
		return "", 0, err
	}
	defer raw.Close()

	originalHash := sha256.New()
	input := bufio.NewReader(io.TeeReader(io.LimitReader(raw, rollout.Bytes+1), originalHash))
	first, err := readFirstRecord(input, maxSessionMetaBytes)
	if err != nil {
		return "", 0, err
	}
	mapped, err := rewriteSessionMetaCWD(first, rollout.ThreadID, targetCWD)
	if err != nil {
		return "", 0, err
	}
	outputHash := sha256.New()
	output := io.MultiWriter(destination, outputHash)
	written, err := output.Write(mapped)
	if err != nil {
		return "", int64(written), err
	}
	rest, err := io.Copy(output, input)
	if err != nil {
		return "", int64(written) + rest, err
	}
	consumed := int64(len(first)) + rest
	if consumed != rollout.Bytes || hex.EncodeToString(originalHash.Sum(nil)) != rollout.SHA256 {
		return "", 0, errors.New("迁移包条目在执行前未通过完整性复核")
	}
	return hex.EncodeToString(outputHash.Sum(nil)), int64(written) + rest, nil
}

func CopyMappedClaudeTranscript(bundlePath string, transcript ImportTranscript, targetCWD string, destination io.Writer) (string, int64, error) {
	if transcript.Tool != "claude" || targetCWD == "" || !filepath.IsAbs(targetCWD) {
		return "", 0, errors.New("Claude Code 原生导入参数无效")
	}
	reader, err := zip.OpenReader(filepath.Clean(bundlePath))
	if err != nil {
		return "", 0, fmt.Errorf("重新打开迁移包失败: %w", err)
	}
	defer reader.Close()
	var file *zip.File
	for _, candidate := range reader.File {
		if candidate.Name == transcript.EntryPath {
			file = candidate
			break
		}
	}
	if file == nil || file.UncompressedSize64 != uint64(transcript.Bytes) {
		return "", 0, errors.New("迁移包在执行前发生变化")
	}
	raw, err := file.Open()
	if err != nil {
		return "", 0, err
	}
	defer raw.Close()
	inputHash := sha256.New()
	outputHash := sha256.New()
	input := bufio.NewReaderSize(io.TeeReader(io.LimitReader(raw, transcript.Bytes+1), inputHash), 64*1024)
	output := io.MultiWriter(destination, outputHash)
	var consumed, written int64
	seen := false
	for {
		line, readErr := readJSONLRecord(input, maxJSONLRecordBytes)
		consumed += int64(len(line))
		if len(bytes.TrimSpace(line)) > 0 {
			rewritten, matched, rewriteErr := rewriteClaudeRecordCWD(line, transcript.SessionID, targetCWD)
			if rewriteErr != nil {
				return "", written, rewriteErr
			}
			seen = seen || matched
			count, writeErr := output.Write(rewritten)
			written += int64(count)
			if writeErr != nil {
				return "", written, writeErr
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", written, readErr
		}
	}
	if !seen {
		return "", written, errors.New("Claude Code 会话缺少匹配的 sessionId")
	}
	if consumed != transcript.Bytes || hex.EncodeToString(inputHash.Sum(nil)) != transcript.SHA256 {
		return "", 0, errors.New("迁移包条目在执行前未通过完整性复核")
	}
	return hex.EncodeToString(outputHash.Sum(nil)), written, nil
}

func readJSONLRecord(reader *bufio.Reader, limit int) ([]byte, error) {
	line := make([]byte, 0, 4096)
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > limit {
			return nil, errors.New("Claude Code 会话记录超过安全上限")
		}
		line = append(line, fragment...)
		if err == nil {
			return line, nil
		}
		if errors.Is(err, io.EOF) {
			return line, io.EOF
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func rewriteClaudeRecordCWD(line []byte, sessionID, targetCWD string) ([]byte, bool, error) {
	trimmed := bytes.TrimSpace(line)
	var record map[string]json.RawMessage
	if json.Unmarshal(trimmed, &record) != nil {
		return nil, false, errors.New("Claude Code 会话包含无效 JSON 记录")
	}
	matched := false
	if rawID, ok := record["sessionId"]; ok {
		var id string
		if json.Unmarshal(rawID, &id) == nil && id != "" && id != sessionID {
			return nil, false, errors.New("Claude Code 会话身份在执行前发生变化")
		}
		matched = id == sessionID
	}
	if _, ok := record["cwd"]; ok {
		record["cwd"], _ = json.Marshal(filepath.Clean(targetCWD))
	}
	rewritten, err := json.Marshal(record)
	if err != nil {
		return nil, false, err
	}
	if bytes.HasSuffix(line, []byte("\r\n")) {
		rewritten = append(rewritten, '\r', '\n')
	} else if bytes.HasSuffix(line, []byte("\n")) {
		rewritten = append(rewritten, '\n')
	}
	return rewritten, matched, nil
}

type sessionMeta struct {
	ID        string
	Timestamp string
	CWD       string
}

func readSessionMeta(file *zip.File) (sessionMeta, error) {
	reader, err := file.Open()
	if err != nil {
		return sessionMeta{}, err
	}
	defer reader.Close()
	first, err := readFirstRecord(bufio.NewReader(io.LimitReader(reader, maxSessionMetaBytes+1)), maxSessionMetaBytes)
	if err != nil {
		return sessionMeta{}, err
	}
	return decodeSessionMeta(first)
}

func decodeSessionMeta(line []byte) (sessionMeta, error) {
	var wrapper struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		Payload   struct {
			ID        string `json:"id"`
			Timestamp string `json:"timestamp"`
			CWD       string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &wrapper); err != nil || wrapper.Type != "session_meta" {
		return sessionMeta{}, errors.New("完整会话首条记录不是有效的 session_meta")
	}
	timestamp := wrapper.Payload.Timestamp
	if timestamp == "" {
		timestamp = wrapper.Timestamp
	}
	return sessionMeta{ID: strings.TrimSpace(wrapper.Payload.ID), Timestamp: timestamp, CWD: strings.TrimSpace(wrapper.Payload.CWD)}, nil
}

func rewriteSessionMetaCWD(line []byte, threadID, targetCWD string) ([]byte, error) {
	meta, err := decodeSessionMeta(line)
	if err != nil {
		return nil, err
	}
	if meta.ID != threadID {
		return nil, errors.New("完整会话线程身份在执行前发生变化")
	}
	if targetCWD == "" || !filepath.IsAbs(targetCWD) {
		return nil, errors.New("目标项目目录必须是绝对路径")
	}
	if samePath(meta.CWD, targetCWD) {
		return line, nil
	}
	trimmed := bytes.TrimSpace(line)
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return nil, err
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(wrapper["payload"], &payload); err != nil {
		return nil, err
	}
	cwd, err := json.Marshal(filepath.Clean(targetCWD))
	if err != nil {
		return nil, err
	}
	payload["cwd"] = cwd
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	wrapper["payload"] = payloadData
	mapped, err := json.Marshal(wrapper)
	if err != nil {
		return nil, err
	}
	if bytes.HasSuffix(line, []byte("\r\n")) {
		return append(mapped, '\r', '\n'), nil
	}
	if bytes.HasSuffix(line, []byte("\n")) {
		return append(mapped, '\n'), nil
	}
	return mapped, nil
}

func readFirstRecord(reader *bufio.Reader, limit int) ([]byte, error) {
	line := make([]byte, 0, 4096)
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > limit {
			return nil, errors.New("session_meta 记录超过安全上限")
		}
		line = append(line, fragment...)
		if err == nil || errors.Is(err, io.EOF) {
			if len(line) == 0 {
				return nil, errors.New("完整会话为空")
			}
			return line, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func verifyZipEntry(file *zip.File, entry manifestEntry) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	hasher := sha256.New()
	read, err := io.Copy(hasher, io.LimitReader(reader, entry.Bytes+1))
	if err != nil {
		return err
	}
	if read != entry.Bytes {
		return fmt.Errorf("迁移包条目 %q 大小校验失败", entry.Path)
	}
	if hex.EncodeToString(hasher.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("迁移包条目 %q 完整性校验失败", entry.Path)
	}
	return nil
}

func readZipEntryCapped(file *zip.File, limit int64, label string) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("%s超过安全上限", label)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return readCapped(reader, limit, label)
}

func readCapped(reader io.Reader, limit int64, label string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s超过安全上限", label)
	}
	return data, nil
}

func validateArchivePath(name string) error {
	if name == "" || strings.ContainsRune(name, '\\') || pathpkg.IsAbs(name) {
		return fmt.Errorf("迁移包包含不安全路径 %q", name)
	}
	cleaned := pathpkg.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != strings.TrimSuffix(name, "/") {
		if !strings.HasSuffix(name, "/") || cleaned != strings.TrimSuffix(name, "/") {
			return fmt.Errorf("迁移包包含不安全路径 %q", name)
		}
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if strings.HasPrefix(segment, ".") {
			return fmt.Errorf("迁移包包含隐藏条目 %q", name)
		}
	}
	return nil
}

func validationMessage(err error) string {
	if err == nil {
		return "文件未损坏"
	}
	message := err.Error()
	if strings.Contains(message, "完整性") || strings.Contains(message, "摘要") || strings.Contains(message, "大小") {
		return "迁移包完整性校验失败"
	}
	if strings.Contains(message, "安全上限") || strings.Contains(message, "不安全路径") || strings.Contains(message, "隐藏条目") || strings.Contains(message, "未在清单声明") {
		return "迁移包未通过安全检查"
	}
	return "迁移包无效或已损坏"
}

func parseSessionTime(values ...string) time.Time {
	for _, value := range values {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
