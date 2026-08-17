package codex

import (
	"bufio"
	"bytes"
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
	maxRecordBytes  = 4 << 20
	maxTextBytes    = 256 << 10
	maxStateBytes   = 16 << 20
	recentGroupName = "最近"
)

type sessionFile struct {
	path    string
	size    int64
	modTime time.Time
}

type visibleThread struct {
	projectID   string
	projectName string
}

// RolloutSource 是后端导出所需的完整会话来源，不会暴露给前端。
type RolloutSource struct {
	Conversation domain.ConversationSummary
	Path         string
	FileName     string
}

type projectGroup struct {
	key           string
	name          string
	latestUpdated string
	conversations []domain.ConversationSummary
}

type codexSidebarState struct {
	ThreadOrders map[string]struct {
		ThreadIDs []string `json:"threadIds"`
	} `json:"sidebar-project-thread-orders"`
	ThreadAssignments map[string]struct {
		ProjectID string `json:"projectId"`
	} `json:"thread-project-assignments"`
	LocalProjects map[string]struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		RootPaths []string `json:"rootPaths"`
	} `json:"local-projects"`
	ProjectlessThreadIDs []string `json:"projectless-thread-ids"`
}

// ScanRoot 做受限只读扫描。它只返回用户可见投影，不返回完整路径或原始 JSON。
func ScanRoot(root string) (domain.ScanResult, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return domain.ScanResult{}, errors.New("Codex 数据根目录必须是绝对路径")
	}
	info, err := os.Stat(root)
	if err != nil {
		return domain.ScanResult{}, fmt.Errorf("无法访问 Codex 数据根目录: %w", err)
	}
	if !info.IsDir() {
		return domain.ScanResult{}, errors.New("Codex 数据根目录不是目录")
	}
	visibleThreads, err := loadVisibleThreads(root)
	if err != nil {
		return domain.ScanResult{}, err
	}
	if len(visibleThreads) == 0 {
		return domain.ScanResult{Source: "codex-sidebar-read-only", Projects: []domain.ProjectSummary{}, Message: "Codex 当前没有可迁移对话"}, nil
	}
	files, total, err := collectSessionFiles(root, visibleThreads)
	if err != nil {
		return domain.ScanResult{}, err
	}
	if len(files) == 0 {
		return domain.ScanResult{Source: "codex-read-only", Projects: []domain.ProjectSummary{}, ScannedBytes: total, Message: "未找到可迁移对话"}, nil
	}
	groups := map[string]*projectGroup{}
	for _, path := range files {
		conversation, ok, err := parseSessionFile(path)
		if err != nil {
			continue
		}
		if !ok {
			continue
		}
		thread, visible := visibleThreads[conversation.ID]
		if !visible {
			continue
		}
		displayGroupName := thread.projectName
		if displayGroupName == "" {
			displayGroupName = conversation.SourceProject
		}
		if displayGroupName == "" {
			displayGroupName = recentGroupName
		}
		if displayGroupName == "." || displayGroupName == string(filepath.Separator) || displayGroupName == "sessions" || displayGroupName == "archived_sessions" {
			displayGroupName = recentGroupName
		}
		if thread.projectID == "" {
			// “最近”只是界面分组，迁移数据继续保留原有的无项目语义。
			conversation.SourceProject = "未命名项目"
		} else {
			conversation.SourceProject = displayGroupName
		}
		// 不把来源完整路径交给 UI；真实导入适配器以后续内部模型承载。
		conversation.SourceDirectory = ""
		groupKey := "project:" + thread.projectID
		if displayGroupName == recentGroupName && thread.projectID == "" {
			groupKey = "recent"
		}
		group := groups[groupKey]
		if group == nil {
			group = &projectGroup{key: groupKey, name: displayGroupName}
			groups[groupKey] = group
		}
		group.conversations = append(group.conversations, conversation)
		if timestampAfter(conversation.UpdatedAt, group.latestUpdated) {
			group.latestUpdated = conversation.UpdatedAt
		}
	}
	orderedGroups := make([]*projectGroup, 0, len(groups))
	for _, group := range groups {
		orderedGroups = append(orderedGroups, group)
	}
	sort.SliceStable(orderedGroups, func(i, j int) bool {
		if orderedGroups[i].key == "recent" || orderedGroups[j].key == "recent" {
			return orderedGroups[i].key == "recent"
		}
		if orderedGroups[i].latestUpdated == orderedGroups[j].latestUpdated {
			return orderedGroups[i].name < orderedGroups[j].name
		}
		return timestampAfter(orderedGroups[i].latestUpdated, orderedGroups[j].latestUpdated)
	})
	result := domain.ScanResult{Source: fmt.Sprintf("codex-read-only:%d-bytes", total), Projects: make([]domain.ProjectSummary, 0, len(orderedGroups)), ScannedBytes: total}
	for index, group := range orderedGroups {
		items := group.conversations
		sort.Slice(items, func(i, j int) bool { return timestampAfter(items[i].UpdatedAt, items[j].UpdatedAt) })
		result.Projects = append(result.Projects, domain.ProjectSummary{ID: fmt.Sprintf("codex-project-%d", index+1), Name: group.name, ConversationCount: len(items), Conversations: items})
	}
	return result, nil
}

// SelectRollouts 只校验已选会话的当前可见性和文件完整性，不重复执行完整侧边栏扫描。
func SelectRollouts(root string, conversationIDs []string) ([]RolloutSource, error) {
	wanted := make(map[string]struct{}, len(conversationIDs))
	for _, id := range conversationIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, errors.New("至少选择一个对话")
	}
	visibleThreads, err := loadVisibleThreads(root)
	if err != nil {
		return nil, err
	}
	files, err := collectSelectedSessionFiles(root, visibleThreads, wanted)
	if err != nil {
		return nil, err
	}
	sources := make([]RolloutSource, 0, len(wanted))
	for _, path := range files {
		id := threadIDFromRolloutName(filepath.Base(path))
		if _, ok := wanted[id]; !ok {
			continue
		}
		conversation, ok, parseErr := parseSessionFile(path)
		if parseErr != nil || !ok || conversation.ID != id {
			continue
		}
		thread, visible := visibleThreads[id]
		if !visible {
			continue
		}
		conversation.SourceProject = displayProjectName(thread, conversation.SourceProject)
		sources = append(sources, RolloutSource{Conversation: conversation, Path: path, FileName: filepath.Base(path)})
	}
	if len(sources) != len(wanted) {
		return nil, errors.New("部分所选对话已不存在，请重新扫描")
	}
	return sources, nil
}

func collectSelectedSessionFiles(root string, visibleThreads map[string]visibleThread, wanted map[string]struct{}) ([]string, error) {
	activeRoot := filepath.Join(root, "sessions")
	if info, err := os.Stat(activeRoot); err != nil || !info.IsDir() {
		return []string{}, nil
	}
	files := make([]string, 0, len(wanted))
	err := filepath.WalkDir(activeRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(strings.ToLower(name), ".jsonl") {
			return nil
		}
		id := threadIDFromRolloutName(name)
		if _, wanted := wanted[id]; !wanted {
			return nil
		}
		if _, visible := visibleThreads[id]; !visible {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func displayProjectName(thread visibleThread, sessionProject string) string {
	if thread.projectID == "" {
		return "未命名项目"
	}
	name := thread.projectName
	if name == "" {
		name = sessionProject
	}
	if name == "" || name == "." || name == string(filepath.Separator) || name == "sessions" || name == "archived_sessions" {
		return recentGroupName
	}
	return name
}

func timestampAfter(left, right string) bool {
	if right == "" {
		return left != ""
	}
	leftTime, leftErr := time.Parse(time.RFC3339Nano, left)
	rightTime, rightErr := time.Parse(time.RFC3339Nano, right)
	if leftErr == nil && rightErr == nil {
		return leftTime.After(rightTime)
	}
	return left > right
}

func loadVisibleThreads(root string) (map[string]visibleThread, error) {
	path := filepath.Join(root, ".codex-global-state.json")
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("无法读取 Codex 当前对话列表，请先打开 Codex 后重试")
		}
		return nil, fmt.Errorf("无法读取 Codex 当前对话列表: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("Codex 当前对话列表不是安全的常规文件")
	}
	if info.Size() > maxStateBytes {
		return nil, errors.New("Codex 当前对话列表超过安全读取限制")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取 Codex 当前对话列表: %w", err)
	}
	var state codexSidebarState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, errors.New("Codex 当前对话列表格式暂不兼容，请更新迁移工具后重试")
	}
	projectNames := make(map[string]string, len(state.LocalProjects))
	for key, project := range state.LocalProjects {
		projectID := strings.TrimSpace(project.ID)
		if projectID == "" {
			projectID = key
		}
		projectNames[projectID] = strings.TrimSpace(project.Name)
	}
	threads := make(map[string]visibleThread)
	for projectID, order := range state.ThreadOrders {
		for _, threadID := range order.ThreadIDs {
			threadID = strings.TrimSpace(threadID)
			if threadID == "" || strings.HasPrefix(threadID, "client-new-thread:") {
				continue
			}
			assignedProjectID := projectID
			if assignment, ok := state.ThreadAssignments[threadID]; ok && assignment.ProjectID != "" {
				assignedProjectID = assignment.ProjectID
			}
			threads[threadID] = visibleThread{projectID: assignedProjectID, projectName: projectNames[assignedProjectID]}
		}
	}
	for _, threadID := range state.ProjectlessThreadIDs {
		threadID = strings.TrimSpace(threadID)
		if threadID != "" && !strings.HasPrefix(threadID, "client-new-thread:") {
			threads[threadID] = visibleThread{projectName: recentGroupName}
		}
	}
	return threads, nil
}

func collectSessionFiles(root string, visibleThreads map[string]visibleThread) ([]string, int64, error) {
	candidates := make([]sessionFile, 0)
	// 归档 rollout 不属于当前工作区，产品导出入口只展示活动 sessions。
	activeRoot := filepath.Join(root, "sessions")
	if info, err := os.Stat(activeRoot); err != nil || !info.IsDir() {
		return []string{}, 0, nil
	}
	err := filepath.WalkDir(activeRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(strings.ToLower(name), ".jsonl") {
			return nil
		}
		if threadID := threadIDFromRolloutName(entry.Name()); threadID != "" {
			if _, visible := visibleThreads[threadID]; !visible {
				return nil
			}
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		candidates = append(candidates, sessionFile{path: path, size: info.Size(), modTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime.After(candidates[j].modTime) })
	files := make([]string, 0, len(candidates))
	var total int64
	for _, candidate := range candidates {
		files = append(files, candidate.path)
		total += candidate.size
	}
	return files, total, nil
}

func threadIDFromRolloutName(name string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".jsonl"), ".json")
	const uuidLength = 36
	if !strings.HasPrefix(base, "rollout-") || len(base) < uuidLength {
		return ""
	}
	candidate := base[len(base)-uuidLength:]
	if strings.Count(candidate, "-") != 4 {
		return ""
	}
	return candidate
}

type parsedMessage struct{ role, text, timestamp string }

func parseSessionFile(path string) (domain.ConversationSummary, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return domain.ConversationSummary{}, false, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return domain.ConversationSummary{}, false, err
	}
	var user, assistant parsedMessage
	id := threadIDFromRolloutName(filepath.Base(path))
	var latestTimestamp string
	var projectName string
	reader := bufio.NewReaderSize(file, 64<<10)
	for {
		line, oversized, readErr := readLineBounded(reader, maxRecordBytes)
		if oversized {
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return domain.ConversationSummary{}, false, readErr
			}
			continue
		}
		if len(line) > 0 && !bytes.Contains(line, []byte(`"type":"session_meta"`)) && !bytes.Contains(line, []byte(`"type":"event_msg"`)) {
			if readErr == io.EOF {
				break
			}
			continue
		}
		var value any
		if len(line) > 0 && json.Unmarshal(line, &value) == nil {
			object, ok := value.(map[string]any)
			if !ok {
				continue
			}
			recordType := firstString(object, "type")
			payload, _ := object["payload"].(map[string]any)
			switch recordType {
			case "session_meta":
				if id == "" {
					id = firstString(payload, "id")
				}
				if projectName == "" {
					projectName = projectNameFromSessionMeta(object)
				}
			case "event_msg":
				collectEventMessage(payload, &user, &assistant)
			}
			if timestamp := firstString(object, "timestamp", "updated_at", "created_at"); timestamp != "" {
				latestTimestamp = timestamp
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return domain.ConversationSummary{}, false, readErr
		}
	}
	if id == "" {
		return domain.ConversationSummary{}, false, nil
	}
	updated := latestTimestamp
	if updated == "" {
		updated = stat.ModTime().UTC().Format(time.RFC3339)
	}
	return domain.ConversationSummary{ID: id, Title: titleFrom(user.text), UpdatedAt: updated, UserMessage: user.text, FinalReply: assistant.text, SourceProject: projectName}, true, nil
}

func readLineBounded(reader *bufio.Reader, limit int) ([]byte, bool, error) {
	line := make([]byte, 0, min(limit, 64<<10))
	oversized := false
	for {
		fragment, isPrefix, err := reader.ReadLine()
		if err != nil {
			return line, oversized, err
		}
		if !oversized {
			if len(line)+len(fragment) > limit {
				oversized = true
				line = nil
			} else {
				line = append(line, fragment...)
			}
		}
		if !isPrefix {
			return line, oversized, nil
		}
	}
}

func projectNameFromSessionMeta(object map[string]any) string {
	if firstString(object, "type") != "session_meta" {
		return ""
	}
	payload, ok := object["payload"].(map[string]any)
	if !ok {
		return ""
	}
	cwd := firstString(payload, "cwd")
	if cwd == "" {
		return ""
	}
	name := filepath.Base(filepath.Clean(cwd))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return ""
	}
	return name
}

func collectEventMessage(payload map[string]any, user, assistant *parsedMessage) {
	eventType := strings.ToLower(firstString(payload, "type"))
	switch eventType {
	case "user_message":
		text := visibleUserText(normalizeText(payload["message"]))
		if text == "" {
			return
		}
		if user.text == "" {
			*user = parsedMessage{role: "user", text: text}
		}
	case "agent_message":
		text := normalizeText(payload["message"])
		if text == "" {
			return
		}
		*assistant = parsedMessage{role: "assistant", text: text}
	}
}

func visibleUserText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || isInjectedContext(trimmed) {
		return ""
	}
	if strings.HasPrefix(trimmed, "# Files mentioned by the user:") {
		lines := strings.Split(trimmed, "\n")
		for index, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "## My request") {
				request := strings.TrimSpace(strings.Join(lines[index+1:], "\n"))
				if request != "" {
					return request
				}
				return "包含附件的对话"
			}
		}
		return "包含附件的对话"
	}
	return trimmed
}

// VisibleUserText 复用扫描器的用户可见消息过滤规则，供跨工具转换使用。
func VisibleUserText(text string) string {
	return visibleUserText(text)
}

func isInjectedContext(text string) bool {
	trimmed := strings.TrimSpace(text)
	for _, prefix := range []string{
		"# AGENTS.md instructions",
		"# Context from my IDE setup:",
		"<environment_context>",
		"<codex_delegation>",
		"<app-context>",
		"<skills_instructions>",
		"<permissions instructions>",
		"<collaboration_mode>",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	// 某些版本会在注入内容前加一小段说明，但仍保留这些稳定标记。
	for _, marker := range []string{
		"<workspace_roots>",
		"<permission_profile",
		"<INSTRUCTIONS>",
	} {
		if strings.Contains(trimmed, marker) {
			return true
		}
	}
	return false
}

func textFrom(object map[string]any) string {
	for _, key := range []string{"text", "message", "content"} {
		if value, ok := object[key]; ok {
			if text := normalizeText(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizeText(value any) string {
	switch typed := value.(type) {
	case string:
		if len(typed) > maxTextBytes {
			return typed[:maxTextBytes]
		}
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := normalizeText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		return textFrom(typed)
	default:
		return ""
	}
}

func firstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func titleFrom(text string) string {
	line := strings.SplitN(strings.TrimSpace(text), "\n", 2)[0]
	if len([]rune(line)) > 48 {
		return string([]rune(line)[:48]) + "…"
	}
	if line == "" {
		return "未命名对话"
	}
	return line
}
