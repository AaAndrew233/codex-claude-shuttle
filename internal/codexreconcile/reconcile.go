package codexreconcile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout = 20 * time.Second
	maxMessageSize = 16 << 20
	maxStderrBytes = 64 << 10
)

var threadIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var commandContext = exec.CommandContext

type Options struct {
	CodexRoot  string
	ThreadIDs  []string
	Executable string
	Timeout    time.Duration
}

type Result struct {
	Version  string
	Verified []string
	Warnings []string
}

func Reconcile(parent context.Context, options Options) (Result, error) {
	root := filepath.Clean(strings.TrimSpace(options.CodexRoot))
	if !filepath.IsAbs(root) {
		return Result{}, errors.New("Codex 数据根目录必须是绝对路径")
	}
	ids, err := validatedIDs(options.ThreadIDs)
	if err != nil {
		return Result{}, err
	}
	if len(ids) == 0 {
		return Result{}, nil
	}
	executable := strings.TrimSpace(options.Executable)
	if executable == "" {
		executable, err = FindExecutable()
		if err != nil {
			return Result{}, err
		}
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	client, err := startClient(ctx, executable, root)
	if err != nil {
		return Result{}, err
	}
	defer client.close()
	version, reportedRoot, err := client.initialize()
	if err != nil {
		return Result{}, err
	}
	if !samePath(root, reportedRoot) {
		return Result{}, errors.New("Codex 原生服务打开了错误的数据目录")
	}
	for _, id := range ids {
		if err := client.readThread(id); err != nil {
			return Result{Version: version}, fmt.Errorf("Codex 未能读取导入会话 %s: %w", id, err)
		}
	}
	discovered, err := client.listThreads()
	if err != nil {
		return Result{Version: version}, err
	}
	result := Result{Version: version}
	for _, id := range ids {
		if discovered[id] {
			result.Verified = append(result.Verified, id)
		}
	}
	if len(result.Verified) != len(ids) {
		return result, fmt.Errorf("Codex 仍有 %d 个导入会话未被发现", len(ids)-len(result.Verified))
	}
	return result, nil
}

func FindExecutable() (string, error) {
	if candidate, err := exec.LookPath("codex"); err == nil {
		return filepath.Abs(candidate)
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/ChatGPT.app/Contents/Resources/codex",
			"/Applications/Codex.app/Contents/Resources/codex",
		)
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			candidates = append(candidates,
				filepath.Join(local, "Programs", "ChatGPT", "resources", "codex.exe"),
				filepath.Join(local, "Programs", "Codex", "resources", "codex.exe"),
			)
		}
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", errors.New("未找到 Codex 原生服务；文件已导入，但需要重启 Codex 后确认发现结果")
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC %d: %s", e.Code, e.Message)
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type client struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	stderr  *boundedBuffer
	nextID  int
}

func startClient(ctx context.Context, executable, root string) (*client, error) {
	command := commandContext(ctx, executable, "app-server", "--stdio")
	command.Env = replaceEnvironment(os.Environ(), "CODEX_HOME", root)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &boundedBuffer{}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("启动 Codex 原生服务失败: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), maxMessageSize)
	return &client{command: command, stdin: stdin, scanner: scanner, stderr: stderr}, nil
}

func (client *client) initialize() (string, string, error) {
	var result struct {
		UserAgent string `json:"userAgent"`
		CodexHome string `json:"codexHome"`
	}
	if err := client.call("initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "codex-claude-shuttle", "version": "0.1.1"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}, &result); err != nil {
		return "", "", fmt.Errorf("初始化 Codex 原生服务失败: %w", err)
	}
	if err := client.notify("initialized", nil); err != nil {
		return "", "", err
	}
	return result.UserAgent, result.CodexHome, nil
}

func (client *client) readThread(id string) error {
	var ignored json.RawMessage
	return client.call("thread/read", map[string]any{"threadId": id, "includeTurns": false}, &ignored)
}

func (client *client) listThreads() (map[string]bool, error) {
	resultIDs := make(map[string]bool)
	var cursor any
	seenCursors := make(map[string]bool)
	for page := 0; page < 10_000; page++ {
		params := map[string]any{"limit": 100}
		if cursor != nil {
			params["cursor"] = cursor
		}
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			NextCursor any `json:"nextCursor"`
		}
		if err := client.call("thread/list", params, &result); err != nil {
			return nil, fmt.Errorf("Codex 无法列出导入会话: %w", err)
		}
		for _, item := range result.Data {
			if threadIDPattern.MatchString(item.ID) {
				resultIDs[item.ID] = true
			}
		}
		if result.NextCursor == nil {
			return resultIDs, nil
		}
		key := fmt.Sprint(result.NextCursor)
		if key == "" || seenCursors[key] {
			return nil, errors.New("Codex 会话列表分页状态无效")
		}
		seenCursors[key] = true
		cursor = result.NextCursor
	}
	return nil, errors.New("Codex 会话列表超过分页安全上限")
}

func (client *client) call(method string, params any, output any) error {
	client.nextID++
	id := client.nextID
	if err := client.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return err
	}
	for client.scanner.Scan() {
		var response rpcResponse
		if json.Unmarshal(client.scanner.Bytes(), &response) != nil || response.ID != id {
			continue
		}
		if response.Error != nil {
			return response.Error
		}
		if output == nil || len(response.Result) == 0 || bytes.Equal(response.Result, []byte("null")) {
			return nil
		}
		if raw, ok := output.(*json.RawMessage); ok {
			*raw = append((*raw)[:0], response.Result...)
			return nil
		}
		return json.Unmarshal(response.Result, output)
	}
	if err := client.scanner.Err(); err != nil {
		return fmt.Errorf("读取 Codex 原生服务响应失败: %w", err)
	}
	return fmt.Errorf("Codex 原生服务提前退出%s", client.stderrSuffix())
}

func (client *client) notify(method string, params any) error {
	message := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		message["params"] = params
	}
	return client.write(message)
}

func (client *client) write(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = client.stdin.Write(data)
	return err
}

func (client *client) close() {
	_ = client.stdin.Close()
	done := make(chan struct{})
	go func() {
		_ = client.command.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		if client.command.Process != nil {
			_ = client.command.Process.Kill()
		}
		<-done
	}
}

func (client *client) stderrSuffix() string {
	text := strings.TrimSpace(client.stderr.String())
	if text == "" {
		return ""
	}
	return ": " + text
}

type boundedBuffer struct {
	mutex sync.Mutex
	data  []byte
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	written := len(value)
	buffer.data = append(buffer.data, value...)
	if len(buffer.data) > maxStderrBytes {
		buffer.data = append([]byte(nil), buffer.data[len(buffer.data)-maxStderrBytes:]...)
	}
	return written, nil
}

func (buffer *boundedBuffer) String() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return string(buffer.data)
}

func validatedIDs(values []string) ([]string, error) {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !threadIDPattern.MatchString(value) {
			return nil, errors.New("迁移包包含无效的 Codex 会话 ID")
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func replaceEnvironment(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(strings.ToUpper(entry), prefix) {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}

func samePath(left, right string) bool {
	left = canonicalPath(left)
	right = canonicalPath(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func canonicalPath(value string) string {
	value = filepath.Clean(value)
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return filepath.Clean(resolved)
	}
	return value
}
