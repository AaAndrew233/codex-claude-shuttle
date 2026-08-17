# ADR 0001: Desktop Stack

状态：Accepted  
日期：2026-08-11

## Context

产品需要在 macOS 和 Windows 上提供无需命令行、账号、云服务或本地网页服务的桌面体验。核心工作是安全处理大量本地会话文件，而不是复杂网络业务。

## Decision

采用：

- Wails 2.14.0 作为首个验证版本。
- Go 1.25.x 作为核心工具链。
- Svelte 与 TypeScript 作为前端。
- pnpm 管理并锁定前端依赖。

框架版本必须精确锁定。升级需要通过 Go 测试、前端测试、macOS 原生构建与冒烟、Windows 原生构建与冒烟，不自动追随最新版本。Wails 3 在正式稳定且迁移收益经过评估前不采用。

## Rationale

- Wails 使用系统 WebView，可生成正常桌面应用且生产环境不需要本地服务。
- Go 适合流式文件处理、受限并发、校验、原子文件操作和跨平台单体分发。
- Wails 与 Go 直接绑定，避免 Rust/Go、Dart/Go 或独立子进程边界。
- Svelte 适合页面数量少但状态较多的桌面工作区，Wails 提供官方 TypeScript 模板。
- TypeScript 约束前后端展示契约，但不承担文件系统或安全决策。

## Alternatives

- Tauri 2 + Rust：安全能力和体积优秀，但会引入 Go 重写或双语言核心。
- Flutter + Dart：渲染一致，但文件核心复用和桌面系统边界成本更高。
- Electron + TypeScript：生态成熟，但捆绑运行时、资源占用和攻击面更大。
- C# + Avalonia：可行，但当前跨平台分发与 Go 候选资产的综合收益较低。

## Consequences

- 必须分别验证 WebKit 与 WebView2 的布局、文件选择、权限和打包行为。
- 前端必须保持薄层，所有敏感文件操作在 Go 后端完成。
- Node.js 只用于开发和构建，不能成为用户运行时依赖。
- `cct` 的 Go 实现仍需独立审计，技术栈一致不代表可以直接复用。

