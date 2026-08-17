# Architecture

## Decision Summary

应用采用 Wails 2.14.0、Go 1.25.x 与 Svelte/TypeScript。Wails 负责桌面生命周期和受限系统桥接，Go 负责所有文件与迁移逻辑，前端只消费经过筛选的展示模型。

Node.js 和 pnpm 仅用于开发构建，不是发布应用的运行时依赖。生产应用不启动本地网页服务器。

## Runtime Boundaries

```text
Svelte UI
  -> tool context + typed Desktop API
  -> application use cases
  -> domain policies
  -> selected tool adapter / bundle storage / operating system

Untrusted inputs
  -> size and path limits
  -> structural validation
  -> integrity verification
  -> read-only import plan
  -> explicit confirmation
  -> staged write and recovery journal
```

前端不得读取任意文件、拼接目标路径、执行 Shell 命令或接触原始会话载荷。所有能力通过最小化的桌面 API 暴露。

## Implemented Core Modules

```text
internal/
  domain/         # bundle, conversation, project and operation contracts
  codex/          # Codex discovery, sidebar intersection and selection
  claude/         # Claude Code project discovery and transcript selection
  bundle/         # versioned package, bounded validation and streamed payloads
  handoff/        # explicit Codex <-> Claude visible-turn conversion
  transferimport/ # four-path preflight, staging, backup, commit and recovery
  codexprocess/   # Codex running-state detection
  claudeprocess/  # Claude Code running-state detection
  codexreconcile/ # Codex app-server discovery verification

frontend/src/
  App.svelte
  components/
  lib/
  style.css

testdata/              # synthetic fixtures only
docs/
```

框架生成的 `main.go`、`app.go`、`wails.json` 与 `frontend/` 先保留标准位置，避免为了目录美观改造生成器约定。

## Core Contracts

- `ToolID`: 受控枚举，只允许 `codex` 与 `claude`，不接受任意字符串决定文件系统行为。
- `ConversationSource`: 发现、读取和描述某一对话源的能力；Codex 与 Claude Code 使用独立实现，不共享私有格式解析器。
- `VisibleConversation`: 供 UI 使用的安全投影，只含项目、标题、时间、用户消息和最终回复。
- `ExportPlanner`: 计算选择、预计大小、警告和目标，不执行写入。
- `BundleWriter`: 仅从白名单载荷创建版本化迁移包。
- `NativeImporter`: 同工具导入经过校验的完整原生会话，并由目标工具适配器负责路径映射、冲突、写入与发现验证。
- `CrossToolConverter`: 仅转换版本化标准消息模型支持的用户消息与最终回复；输出损失清单，不接触内部推理、工具调用、子代理、Memory、Skills、配置或附件。
- `CodexImportService`: 解析并完整校验迁移包，按来源 `session_meta.cwd` 生成目标项目映射，以线程 ID 识别冲突，并签发十分钟有效的一次性导入计划。
- `CodexImportExecutor`: 只执行仍有效且输入未变化的计划；只写 `sessions`，只改写 `session_meta.cwd`，使用同卷暂存、替换备份、恢复日志和读回摘要校验。
- `SyntheticHome` / `ImportExecutor`: 保留给旧合成 A/B 回归测试，不再作为产品导入入口。
- `CodexReconciler`: 启动官方 `codex app-server --stdio`，通过 `thread/read` 与分页 `thread/list` 验证发现；失败不伪装为成功，也不回滚已经通过读回校验的文件。
- `CodexProbe`: 当前仅做用户指定根目录的目录级候选探测，返回平台、可读性和候选目录名；不读取正文、凭据、SQLite 或私有索引，也不据此宣称格式兼容。
- `CodexReader`: 对用户明确指定的根目录执行受限只读扫描，只接受带 `session_meta` 和真实 `event_msg.user_message` 的标准 rollout，并与只读侧边栏线程状态求交集；仅返回项目、标题、真实用户消息和最终 assistant 回复，拥有状态文件、文件数量、单文件、总大小和符号链接边界。侧边栏状态格式异常时明确失败，不降级为全量历史扫描；该私有只读契约仍不能证明长期兼容。
- `CodexReader` 的用户投影把无工作区归属的线程放入固定置顶的“最近”分组；项目按最后对话时间倒序，组内对话按更新时间倒序。侧边栏中已有活动 rollout 的对话全部进入用户投影，不设置单文件、累计大小或文件数量的产品筛选上限。解析器按行流式读取，只反序列化会话元数据和可见消息；单条异常记录有独立内存上限，超过后丢弃该记录并继续扫描同一会话。
- `ClaudeReader`: 只读遍历 Claude Desktop `Claude-3p/local-agent-mode-sessions/<account>/<organization>/local_<session>.json`，仅接受字段完整且 `isArchived=false` 的 Code 侧边栏元数据，再按 `sessionId` 与 `cliSessionId` 定位其会话目录中的唯一原生 JSONL。归档元数据、已删除后的孤立目录、重复 ID、缺失状态和符号链接均按失败关闭处理；导出选择阶段重新求交集。元数据只反序列化会话标识、标题、时间和项目文件夹，不读取账号字段，也不读取或修改 `~/.claude.json`、Memory、Skills 或全局配置。
- `Handoff`: 同工具保留原会话 ID；跨工具根据来源工具、来源 ID 和目标工具生成确定性新 UUID，只提取用户可见消息与助手最终文本，拒绝混合 `sessionId`，不迁移内部推理、工具结果或 sidechain。
- `TransferImportService`: 统一处理四条路径，签发十分钟有效的一次性计划；跨工具单会话转换上限为 128 MB，同工具原生载荷保持流式处理。执行前再次检查目标工具、输入摘要、目标文件和符号链接，执行中记录备份与恢复日志，提交后做 SHA-256 读回。
- `ToolContext`: 顶部工具切换的独立上下文；导出选择来源工具，导入先识别迁移包来源工具再选择目标工具。切换来源会重新扫描并清空选择，任何页面都不混合两个工具的数据。

前端已将 `ToolContext`、Claude Code 来源扫描、同工具审阅和跨工具损失确认接入真实桌面 API。跨工具确认后进入项目映射和统一预检，不绕过目标工具关闭、冲突、空间、备份或读回校验门禁。

Claude Code 作为明确的第二个适配器接入，不创建通用插件系统、动态注册表或可执行脚本式转换语言。

## Data Rules

原始会话载荷不发送到前端。需要修改记录路径时，由目标工具对应的版本化重写器执行明确字段变换，不能使用通用字符串替换处理不同工具的私有格式。

迁移包 v2 包含版本化清单、`sourceTool`、原生载荷、条目大小、内容摘要和安全摘要；v1 Codex 包继续只读兼容。同工具迁移使用原生载荷；跨工具转换只从经过校验的原生载荷提取允许的可见消息，并在 UI 展示未转换内容清单。SHA-256 用于发现意外损坏，不提供来源真实性。密码模式尚未实现；未来接入时必须使用成熟的认证加密格式，不设计自有密码学。

## Import Safety

正式导入前要求目标工具关闭；只检测和阻断当前目标工具，不因无关工具运行而拒绝。导入流程为：

1. 只读解析并实施条目数、单文件大小、总展开大小和路径限制。
2. 在写入前完成所有摘要与清单绑定校验。
3. 生成带内容摘要和有效期的不可变导入计划。
4. 检查目标空间并创建同卷暂存目录。
5. 对每个将替换的目标创建可恢复备份和恢复日志。
6. 原子提交单个文件，读回并再次校验。
7. 失败时按恢复日志处理，逐项报告结果。
8. 写入完成后调用目标工具适配器执行可用的原生发现或重启后可见验证，并单独报告结果。

当前产品入口要求每个来源项目映射到目标设备上已存在的安全目录；既可以选择当前工具已发现的项目，也可以通过系统原生文件夹选择器指定目录。Claude 导出的来源目录是 Claude Desktop 侧边栏存储，Claude 导入目标仍是 `~/.claude` 原生会话目录，两者使用独立 API，不能混用。Claude Code 目标路径按其项目目录编码规则计算并验证，Codex 目标路径只进入 `sessions`。自动创建空项目目录尚未开放。不复制源码，不直接写 Codex SQLite、Claude 全局配置或任何工具的私有索引。

多个文件无法依靠一次重命名获得整体原子性，因此必须使用恢复日志，不能在界面中承诺不存在的“全包原子导入”。

## Configuration and Logging

配置从单一强类型入口读取，只保存语言、主题、窗口状态、最近目录和可重建缓存策略。密码、密钥、Token、Cookie 和真实消息内容永不写入配置或日志。

日志记录阶段、错误码、匿名操作 ID、条目数量和耗时，不记录消息正文、完整路径、迁移包内容或凭据。日志按大小轮转并支持清除。

## Distribution

macOS 与 Windows 使用各自原生 CI Runner 构建和测试。发布前必须具备依赖锁定、许可证清单、SBOM、macOS 签名与 notarization、Windows 代码签名、安装与卸载测试以及升级恢复说明。
