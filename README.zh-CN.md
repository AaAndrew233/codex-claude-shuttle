# Codex Claude Shuttle

[English](README.md)

一个非官方、本地优先的桌面应用，用于导出、导入和迁移 OpenAI Codex 与 Anthropic Claude Code 的对话。

无需云服务、无需部署服务器、无需注册产品账号，正常使用也无需命令行。

> Codex Claude Shuttle 是独立开源项目，与 OpenAI、Anthropic 没有隶属、赞助或官方认可关系。

## 当前状态

Codex Claude Shuttle 目前是源码预览版，尚未发布预编译安装包。

- macOS Apple 芯片版本目前仅完成本地自签名，尚未通过 Apple 公证。
- Windows x64 版本已通过 CI 构建，但尚未完成 Windows 实体机安装与迁移验收。
- 暂不提供 Intel Mac 和 Linux 版本。

首个公开版本准备好后，安装包和 SHA-256 校验值会发布到仓库的 Releases 页面。在此之前，只建议具备源码构建能力的用户使用合成或非关键对话进行测试。

## 主要功能

- 只读取 Codex 或 Claude Code 侧边栏中仍处于活动状态的对话，排除归档、已删除、孤立和状态不明的记录。
- 按项目浏览、搜索和选择对话，导出前可以预览用户消息与助手最终回复。
- 生成包含清单和 SHA-256 完整性校验的本地 ZIP 迁移包。
- 同工具迁移时保留经过校验的原生对话，只按目标电脑的项目目录调整必要路径字段。
- Codex 与 Claude Code 跨工具转换前会展示转换边界，并要求确认后再迁移可见对话文本。
- 目标工具运行时禁止写入；导入前检查冲突、磁盘空间和目标路径，替换时创建备份，提交后逐文件校验。
- 对话数据始终在当前电脑本地处理。

## 支持的迁移方向

| 来源 | 目标 | 结果 |
| --- | --- | --- |
| Codex | Codex | 保留经过校验的原生对话，只更新目标项目路径字段 |
| Claude Code | Claude Code | 保留经过校验的原生对话，只更新目标项目路径字段 |
| Codex | Claude Code | 转换用户可见消息和助手最终可见回复 |
| Claude Code | Codex | 转换用户可见消息和助手最终可见回复 |

跨工具转换不会迁移内部推理、工具调用与结果、子代理、Memory、Skills、附件、认证信息、全局配置或原生运行状态。

## 使用流程

1. 在来源电脑打开“导出对话”，选择 Codex 或 Claude Code。
2. 选择要迁移的项目和对话，核对迁移包内容后保存到本地。
3. 通过可信方式把 ZIP 迁移包移动到目标电脑。
4. 在目标电脑打开“导入对话”，检查迁移包，为每个来源项目选择现有本地项目文件夹，完全退出目标工具，再运行预检并导入。

Codex Claude Shuttle 只移动对话，不复制项目源码，也不会自动创建缺失的项目目录。

## 隐私与安全

Codex Claude Shuttle 不会上传对话数据，不依赖托管后端，也不要求注册产品账号。应用会把迁移包视为不可信输入，在提出任何写入计划前完成结构和完整性检查。

SHA-256 可以发现迁移包内容被改动，但不能证明迁移包由谁创建。当前版本的迁移包尚未加密，请按敏感文件存储和传输。

应用不会写入 Codex SQLite、Claude 全局配置、凭据或私有索引。详细边界和问题上报方式见 [SECURITY.md](SECURITY.md) 与[威胁模型](docs/threat-model.md)。

## 当前限制

- 尚未完成 Apple 公证和商业代码签名。
- Windows 实体机验收尚未完成。
- 自动化写入测试只使用临时目录和合成对话，尚未针对用户当前的 `~/.codex` 或 `~/.claude` 目录完成真实数据写入验收。
- Claude Code 跨设备导入后的继续对话，仍需针对支持的客户端版本完成真实设备验收。
- 暂不支持迁移包加密、自动创建项目目录、应用内更新和 Linux 构建。
- Codex 与 Claude Code 使用的本地私有格式可能变化，请先用非关键对话测试并保留备份。

## 从源码构建

环境要求：Go 1.25.12、Wails 2.14.0、Node.js 22.12 或更高版本、pnpm 10.33.0。

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
go test ./...
go vet ./...
pnpm --dir frontend check
pnpm --dir frontend test
wails build -clean
```

构建产物位于 `build/bin/`。架构说明见 [docs/architecture.md](docs/architecture.md)，桌面技术栈决策见 [docs/adr/0001-desktop-stack.md](docs/adr/0001-desktop-stack.md)。

## 参与贡献

提交 Issue 或 Pull Request 前请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。复现问题时只能使用合成数据，不要上传真实迁移包、对话、完整本地路径、凭据、Token 或 Cookie。

## 许可证

Codex Claude Shuttle 使用 [MIT License](LICENSE) 开源。
