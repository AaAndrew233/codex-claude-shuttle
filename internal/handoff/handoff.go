package handoff

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
)

const (
	maxRecordBytes = 16 << 20
	maxTurnBytes   = 4 << 20
	maxTurns       = 100_000
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Turn struct {
	Role Role
	Text string
}

type Session struct {
	SourceTool string
	ID         string
	CWD        string
	CreatedAt  time.Time
	Turns      []Turn
}

func FromCodex(data []byte) (Session, error) {
	session := Session{SourceTool: "codex"}
	scanner := newScanner(data)
	for scanner.Scan() {
		var wrapper struct {
			Timestamp string          `json:"timestamp"`
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &wrapper) != nil {
			continue
		}
		switch wrapper.Type {
		case "session_meta":
			var meta struct {
				ID        string `json:"id"`
				CWD       string `json:"cwd"`
				Timestamp string `json:"timestamp"`
			}
			if json.Unmarshal(wrapper.Payload, &meta) == nil {
				session.ID = strings.TrimSpace(meta.ID)
				session.CWD = strings.TrimSpace(meta.CWD)
				session.CreatedAt = firstTime(meta.Timestamp, wrapper.Timestamp)
			}
		case "event_msg":
			var event struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if json.Unmarshal(wrapper.Payload, &event) != nil {
				continue
			}
			switch event.Type {
			case "user_message":
				if err := session.add(RoleUser, codex.VisibleUserText(event.Message)); err != nil {
					return Session{}, err
				}
			case "agent_message":
				if err := session.add(RoleAssistant, event.Message); err != nil {
					return Session{}, err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Session{}, err
	}
	return validateSession(session)
}

func FromClaude(data []byte) (Session, error) {
	session := Session{SourceTool: "claude"}
	scanner := newScanner(data)
	for scanner.Scan() {
		var record struct {
			Type        string          `json:"type"`
			SessionID   string          `json:"sessionId"`
			CWD         string          `json:"cwd"`
			Timestamp   string          `json:"timestamp"`
			IsSidechain bool            `json:"isSidechain"`
			Message     json.RawMessage `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.IsSidechain {
			continue
		}
		if session.ID == "" {
			session.ID = strings.TrimSpace(record.SessionID)
		} else if record.SessionID != "" && strings.TrimSpace(record.SessionID) != session.ID {
			return Session{}, errors.New("Claude Code 会话包含不一致的 sessionId")
		}
		if session.CWD == "" {
			session.CWD = strings.TrimSpace(record.CWD)
		}
		if session.CreatedAt.IsZero() {
			session.CreatedAt = firstTime(record.Timestamp)
		}
		if record.Type != "user" && record.Type != "assistant" {
			continue
		}
		text := claudeVisibleText(record.Message)
		role := RoleAssistant
		if record.Type == "user" {
			role = RoleUser
		}
		if err := session.add(role, text); err != nil {
			return Session{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return Session{}, err
	}
	return validateSession(session)
}

func ToCodex(session Session, targetCWD string) ([]byte, string, error) {
	if _, err := validateSession(session); err != nil {
		return nil, "", err
	}
	targetCWD = strings.TrimSpace(targetCWD)
	if targetCWD == "" {
		return nil, "", errors.New("目标项目目录不能为空")
	}
	id := deterministicID(session.SourceTool + ":" + session.ID + ":codex")
	base := session.CreatedAt
	if base.IsZero() {
		base = time.Now().UTC()
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	meta := map[string]any{"id": id, "timestamp": base.Format(time.RFC3339Nano), "cwd": targetCWD, "originator": "conversation-transfer", "cli_version": "conversation-transfer", "source": "cli"}
	if err := encoder.Encode(map[string]any{"timestamp": base.Format(time.RFC3339Nano), "type": "session_meta", "payload": meta}); err != nil {
		return nil, "", err
	}
	for index, turn := range session.Turns {
		timestamp := base.Add(time.Duration(index+1) * time.Millisecond).Format(time.RFC3339Nano)
		eventType := "agent_message"
		blockType := "output_text"
		if turn.Role == RoleUser {
			eventType = "user_message"
			blockType = "input_text"
		}
		event := map[string]any{"timestamp": timestamp, "type": "event_msg", "payload": map[string]any{"type": eventType, "message": turn.Text}}
		if err := encoder.Encode(event); err != nil {
			return nil, "", err
		}
		message := map[string]any{"timestamp": timestamp, "type": "response_item", "payload": map[string]any{"type": "message", "role": string(turn.Role), "content": []map[string]any{{"type": blockType, "text": turn.Text}}}}
		if err := encoder.Encode(message); err != nil {
			return nil, "", err
		}
	}
	return output.Bytes(), id, nil
}

func TargetID(sourceTool, sourceID, targetTool string) (string, error) {
	if sourceTool != "codex" && sourceTool != "claude" || targetTool != "codex" && targetTool != "claude" {
		return "", errors.New("不支持的转换工具")
	}
	if strings.TrimSpace(sourceID) == "" {
		return "", errors.New("来源会话缺少身份标识")
	}
	if sourceTool == targetTool {
		return sourceID, nil
	}
	return deterministicID(sourceTool + ":" + sourceID + ":" + targetTool), nil
}

func ToClaude(session Session, targetCWD string) ([]byte, string, error) {
	if _, err := validateSession(session); err != nil {
		return nil, "", err
	}
	targetCWD = strings.TrimSpace(targetCWD)
	if targetCWD == "" {
		return nil, "", errors.New("目标项目目录不能为空")
	}
	id := deterministicID(session.SourceTool + ":" + session.ID + ":claude")
	base := session.CreatedAt
	if base.IsZero() {
		base = time.Now().UTC()
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	parent := any(nil)
	for index, turn := range session.Turns {
		lineID := deterministicID(fmt.Sprintf("%s:line:%d", id, index))
		content := any(turn.Text)
		if turn.Role == RoleAssistant {
			content = []map[string]any{{"type": "text", "text": turn.Text}}
		}
		record := map[string]any{
			"parentUuid": parent, "isSidechain": false, "type": string(turn.Role), "uuid": lineID,
			"timestamp": base.Add(time.Duration(index) * time.Millisecond).Format(time.RFC3339Nano), "sessionId": id,
			"cwd": targetCWD, "version": "conversation-transfer", "userType": "external", "entrypoint": "conversation-transfer",
			"message": map[string]any{"role": string(turn.Role), "content": content},
		}
		if err := encoder.Encode(record); err != nil {
			return nil, "", err
		}
		parent = lineID
	}
	return output.Bytes(), id, nil
}

func RewriteClaudeCWD(data []byte, sessionID, targetCWD string) ([]byte, error) {
	if strings.TrimSpace(targetCWD) == "" {
		return nil, errors.New("目标项目目录不能为空")
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	var output bytes.Buffer
	seen := false
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			output.Write(line)
			continue
		}
		var record map[string]json.RawMessage
		if json.Unmarshal(trimmed, &record) != nil {
			return nil, errors.New("Claude Code 会话包含无效 JSON 记录")
		}
		if rawID, ok := record["sessionId"]; ok {
			var id string
			if json.Unmarshal(rawID, &id) == nil && id != "" && id != sessionID {
				return nil, errors.New("Claude Code 会话身份在执行前发生变化")
			}
			if id == sessionID {
				seen = true
			}
		}
		if _, ok := record["cwd"]; ok {
			record["cwd"], _ = json.Marshal(targetCWD)
		}
		rewritten, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		output.Write(rewritten)
		if bytes.HasSuffix(line, []byte("\n")) {
			output.WriteByte('\n')
		}
	}
	if !seen {
		return nil, errors.New("Claude Code 会话缺少匹配的 sessionId")
	}
	return output.Bytes(), nil
}

func (session *Session) add(role Role, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) > maxTurnBytes {
		return errors.New("单条可见消息超过跨工具转换上限")
	}
	if len(session.Turns) >= maxTurns {
		return errors.New("可见消息数量超过跨工具转换上限")
	}
	session.Turns = append(session.Turns, Turn{Role: role, Text: text})
	return nil
}

func validateSession(session Session) (Session, error) {
	if strings.TrimSpace(session.ID) == "" {
		return Session{}, errors.New("会话缺少身份标识")
	}
	if len(session.Turns) == 0 {
		return Session{}, errors.New("会话不包含可转换的用户消息或最终回复")
	}
	return session, nil
}

func newScanner(data []byte) *bufio.Scanner {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), maxRecordBytes)
	return scanner
}

func claudeVisibleText(raw json.RawMessage) string {
	var message struct {
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &message) != nil {
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

func firstTime(values ...string) time.Time {
	for _, value := range values {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func deterministicID(value string) string {
	sum := sha256.Sum256([]byte("conversation-transfer:" + value))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}
