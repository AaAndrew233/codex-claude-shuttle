import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'

import App from './App.svelte'

const { applicationInfo, defaultCodexRoot, defaultClaudeRoot, defaultClaudeSidebarRoot, scanCodexRoot, scanClaudeRoot, exportCodex, exportClaude, chooseExportFolder, openExportFolder, chooseImportFile, chooseImportTargetFolder, inspectTransferImport, prepareTransferImport, executeTransferImport } = vi.hoisted(() => ({
  applicationInfo: vi.fn(),
  defaultCodexRoot: vi.fn(),
  defaultClaudeRoot: vi.fn(),
  defaultClaudeSidebarRoot: vi.fn(),
  scanCodexRoot: vi.fn(),
  scanClaudeRoot: vi.fn(),
  exportCodex: vi.fn(),
  exportClaude: vi.fn(),
  chooseExportFolder: vi.fn(),
  openExportFolder: vi.fn(),
  chooseImportFile: vi.fn(),
  chooseImportTargetFolder: vi.fn(),
  inspectTransferImport: vi.fn(),
  prepareTransferImport: vi.fn(),
  executeTransferImport: vi.fn(),
}))

vi.mock('../wailsjs/go/main/App.js', () => ({
  ApplicationInfo: applicationInfo,
  DefaultCodexRoot: defaultCodexRoot,
  DefaultClaudeRoot: defaultClaudeRoot,
  DefaultClaudeSidebarRoot: defaultClaudeSidebarRoot,
  ScanCodexRoot: scanCodexRoot,
  ScanClaudeRoot: scanClaudeRoot,
  ExportCodex: exportCodex,
  ExportClaude: exportClaude,
  ChooseExportFolder: chooseExportFolder,
  OpenExportFolder: openExportFolder,
  ChooseImportFile: chooseImportFile,
  ChooseImportTargetFolder: chooseImportTargetFolder,
  InspectTransferImport: inspectTransferImport,
  PrepareTransferImport: prepareTransferImport,
  ExecuteTransferImport: executeTransferImport,
}))

describe('application startup', () => {
  afterEach(() => {
    cleanup()
    applicationInfo.mockReset()
    defaultCodexRoot.mockReset()
    defaultClaudeRoot.mockReset()
    defaultClaudeSidebarRoot.mockReset()
    scanCodexRoot.mockReset()
    scanClaudeRoot.mockReset()
    exportCodex.mockReset()
    exportClaude.mockReset()
    chooseExportFolder.mockReset()
    openExportFolder.mockReset()
    chooseImportFile.mockReset()
    chooseImportTargetFolder.mockReset()
    inspectTransferImport.mockReset()
    prepareTransferImport.mockReset()
    executeTransferImport.mockReset()
  })

  it('shows an accessible loading state while Codex conversations are scanning', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic/.codex')
    let finishScan: (value: { projects: never[] }) => void = () => {}
    scanCodexRoot.mockImplementation(() => new Promise((resolve) => { finishScan = resolve }))

    render(App)
    const exportButton = await screen.findByRole('button', { name: /Export conversations/ })
    await exportButton.click()

    expect(await screen.findByRole('progressbar', { name: 'Reading Codex conversations' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Scanning…' })).toBeDisabled()

    finishScan({ projects: [] })
    expect(await screen.findByText('No conversations found')).toBeInTheDocument()
  })

  it('shows the confirmed export and import entry screen after loading metadata', async () => {
    applicationInfo.mockResolvedValue({
      name: 'Codex Claude Shuttle',
      version: '0.1.0-dev',
    })

    render(App)

    expect(await screen.findByRole('heading', { name: 'Codex Claude Shuttle' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Export conversations/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Import conversations/ })).toBeInTheDocument()
    expect(screen.getByText(/Project source, account, and global settings are not included/)).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Language' })).toHaveClass('global-language-switch')

    await screen.getByRole('button', { name: '中文' }).click()

    expect(screen.getByRole('button', { name: /导出对话/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /导入对话/ })).toBeInTheDocument()
    expect(screen.getByText('只迁移对话，不包含项目源码、账号和全局设置。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '中文' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows a recoverable startup error when the desktop API is unavailable', async () => {
    applicationInfo.mockRejectedValue(new Error('desktop API unavailable'))

    render(App)

    expect(await screen.findByRole('heading', { name: '无法启动' })).toBeInTheDocument()
  })

  it('keeps the selected language when navigating from home into export', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic/.codex')
    scanCodexRoot.mockResolvedValue({ projects: [] })

    render(App)
    await screen.findByRole('heading', { name: 'Codex Claude Shuttle' })
    await screen.getByRole('button', { name: '中文' }).click()
    await screen.getByRole('button', { name: /导出对话/ }).click()

    expect(await screen.findByRole('heading', { name: '选择对话' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '重新扫描' })).toBeInTheDocument()

    await screen.getByRole('button', { name: 'EN' }).click()
    expect(await screen.findByRole('heading', { name: 'Select conversations' })).toBeInTheDocument()
  })

  it('matches the selected import layout and keeps the language utility separate from the progress navigation', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })

    render(App)
    await screen.findByRole('heading', { name: 'Codex Claude Shuttle' })
    await screen.getByRole('button', { name: /Import conversations/ }).click()

    await screen.findByRole('heading', { name: 'Import conversations' })
    const progressRow = document.querySelector('.import-v6-progress-row')
    const progress = screen.getByRole('navigation', { name: 'Import progress' })
    const languageSwitch = screen.getByRole('group', { name: 'Language' })
    const chineseButton = within(languageSwitch).getByRole('button', { name: '中文' })

    expect(progressRow?.contains(languageSwitch)).toBe(true)
    expect(progress.contains(languageSwitch)).toBe(false)
    expect(languageSwitch).toHaveClass('global-language-switch')
    expect(screen.getByText('INCLUDED')).toBeInTheDocument()
    expect(screen.getByText('NOT INCLUDED')).toBeInTheDocument()
    expect(screen.getByText('BEFORE IMPORT')).toBeInTheDocument()
    expect(screen.getByText('The package is checked before anything is written.')).toBeInTheDocument()
    const importMascot = document.querySelector('.import-v6-mascot')
    expect(importMascot).toHaveAttribute('src', '/assets/import/import-mole-restore.png')
    expect(document.querySelector('.import-v6-heading')?.contains(importMascot)).toBe(true)
    expect(document.querySelector('.import-v6-summary')?.contains(importMascot)).toBe(false)

    await chineseButton.click()
    expect(await screen.findByRole('heading', { name: '导入对话' })).toBeInTheDocument()
    expect(screen.getByText('写入任何内容前都会先检查迁移包。')).toBeInTheDocument()
  })

  it('uses the real import inspection and native Codex process preflight', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic-target/.codex')
    defaultClaudeRoot.mockResolvedValue('/synthetic-target/.claude')
    inspectTransferImport.mockResolvedValue({
      bundlePath: '/synthetic/package.zip', sourceTool: 'codex', projectCount: 1, conversationCount: 2, message: 'ready',
      sources: [{ key: 'source-alpha', name: 'Project Alpha', sourceFolder: 'alpha', conversationCount: 2, suggestedTargetId: 'target-alpha', suggestedTargetName: 'Project Alpha' }],
      targets: [{ id: 'target-alpha', name: 'Project Alpha', folder: 'alpha' }],
    })
    prepareTransferImport.mockResolvedValue({
      planToken: '', requiredBytes: 2048, availableBytes: 1_000_000, projectCount: 1, conversationCount: 2,
      sourceTool: 'codex', targetTool: 'codex', createCount: 2, skipCount: 0, replaceCount: 0, toolRunning: true, canExecute: false, message: 'Codex is running',
    })

    render(App)
    await (await screen.findByRole('button', { name: /Import conversations/ })).click()
    const pathInput = await screen.findByLabelText('Documents/codex-claude-shuttle.zip')
    await fireEvent.input(pathInput, { target: { value: '/synthetic/package.zip' } })
    await screen.getByRole('button', { name: 'Check file' }).click()

    expect(await screen.findByRole('heading', { name: 'Review import' })).toBeInTheDocument()
    expect(screen.getByText('Complete native conversation')).toBeInTheDocument()
    await screen.getByRole('button', { name: 'Continue' }).click()
    expect(await screen.findByText('Transfer package verified')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Project Alpha Choose a target project' })).toHaveValue('target-alpha')
    expect(screen.queryByRole('checkbox', { name: /closed Codex/i })).not.toBeInTheDocument()
    await screen.getByRole('button', { name: 'Run import preflight' }).click()

    expect(await screen.findByText(/Codex is still running/)).toBeInTheDocument()
    expect(prepareTransferImport).toHaveBeenCalledTimes(1)
    expect(prepareTransferImport.mock.calls[0][0]).toMatchObject({
      bundlePath: '/synthetic/package.zip', sourceTool: 'codex', targetTool: 'codex', targetRoot: '/synthetic-target/.codex', conflictPolicy: 'skip',
      mappings: [{ sourceKey: 'source-alpha', targetId: 'target-alpha', targetDirectory: '' }],
    })
  })

  it('shows a dedicated real import result with file and Codex discovery outcomes', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic-target/.codex')
    defaultClaudeRoot.mockResolvedValue('/synthetic-target/.claude')
    inspectTransferImport.mockResolvedValue({
      bundlePath: '/synthetic/package.zip', sourceTool: 'codex', projectCount: 1, conversationCount: 2, message: 'ready',
      sources: [{ key: 'source-alpha', name: 'Project Alpha', sourceFolder: 'alpha', conversationCount: 2, suggestedTargetId: 'target-alpha', suggestedTargetName: 'Project Alpha' }],
      targets: [{ id: 'target-alpha', name: 'Project Alpha', folder: 'alpha' }],
    })
    prepareTransferImport.mockResolvedValue({
      planToken: 'one-time-plan', requiredBytes: 2048, availableBytes: 1_000_000, projectCount: 1, conversationCount: 2,
      sourceTool: 'codex', targetTool: 'codex', createCount: 2, skipCount: 0, replaceCount: 0, toolRunning: false, canExecute: true, message: 'ready',
    })
    executeTransferImport.mockResolvedValue({
      sourceTool: 'codex', targetTool: 'codex',
      written: 2, skipped: 0, replaced: 0, recovered: false, filesVerified: true, discoveryAttempted: true,
      discovered: 1, pendingDiscovery: 1, warnings: ['Restart Codex to refresh the remaining conversation.'], message: 'Files imported',
    })

    render(App)
    await (await screen.findByRole('button', { name: /Import conversations/ })).click()
    const pathInput = await screen.findByLabelText('Documents/codex-claude-shuttle.zip')
    await fireEvent.input(pathInput, { target: { value: '/synthetic/package.zip' } })
    await screen.getByRole('button', { name: 'Check file' }).click()
    await (await screen.findByRole('button', { name: 'Continue' })).click()
    await (await screen.findByRole('button', { name: 'Run import preflight' })).click()
    await (await screen.findByRole('button', { name: 'Start import' })).click()

    expect(await screen.findByRole('heading', { name: 'Conversations imported' })).toBeInTheDocument()
    expect(screen.getByText('Files verified')).toBeInTheDocument()
    expect(screen.getByText('Discovered by Codex')).toBeInTheDocument()
    expect(screen.getByText('Awaiting discovery')).toBeInTheDocument()
    expect(screen.getByText('Restart Codex to refresh the remaining conversation.')).toBeInTheDocument()
    expect(executeTransferImport).toHaveBeenCalledWith('one-time-plan')
    expect(document.querySelector('.export-preview-pane')).toBeNull()
  })

  it('keeps preview focus independent from export selection', async () => {
    const fullPreviewTitle = 'Newest exchange with a complete long title that must never be clipped halfway'
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic/.codex')
    scanCodexRoot.mockResolvedValue({
      projects: [{
        id: 'project-recent',
        name: 'Recent',
        conversationCount: 2,
        conversations: [
          { id: 'conversation-new', title: fullPreviewTitle, updatedAt: '2026-08-14T08:00:00Z', userMessage: 'Newest **request**', finalReply: '### Result\n\nNewest **reply**\n\n```text\nsafe output\n```', sourceProject: 'Recent' },
          { id: 'conversation-old', title: 'Older exchange', updatedAt: '2026-08-13T08:00:00Z', userMessage: 'Older request', finalReply: 'Older reply', sourceProject: 'Recent' },
        ],
      }],
    })

    render(App)
    await (await screen.findByRole('button', { name: /Export conversations/ })).click()

    const previewTitle = await screen.findByRole('heading', { name: fullPreviewTitle })
    expect(previewTitle).toHaveTextContent(fullPreviewTitle)
    const messageRegion = screen.getByRole('region', { name: 'Latest conversation messages' })
    expect(messageRegion).toHaveAttribute('tabindex', '0')
    expect(messageRegion.querySelector('img')).toBeNull()
    expect(document.querySelector('.export-preview-heading-meta .export-preview-mascot')).toBeInTheDocument()
    expect(screen.getByText('request').tagName).toBe('STRONG')
    expect(screen.getByRole('heading', { name: 'Result', level: 3 })).toBeInTheDocument()
    expect(screen.getByText('reply').tagName).toBe('STRONG')
    expect(messageRegion.querySelector('pre code')).toHaveTextContent('safe output')
    const newestCheckbox = screen.getByRole('checkbox', { name: `Select ${fullPreviewTitle}` })
    await newestCheckbox.click()
    expect(newestCheckbox).toBeChecked()
    expect(screen.getByText('1 conversation selected from 1 project')).toBeInTheDocument()

    await screen.getByRole('button', { name: /Older exchange/ }).click()
    expect(await screen.findByRole('heading', { name: 'Older exchange' })).toBeInTheDocument()
    expect(newestCheckbox).toBeChecked()
    expect(screen.getByText('1 conversation selected from 1 project')).toBeInTheDocument()
  })

  it('uses one local 24-hour timestamp format across the export workspace and languages', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic/.codex')
    scanCodexRoot.mockResolvedValue({ projects: [{
      id: 'project-time',
      name: 'Recent',
      conversationCount: 1,
      conversations: [{
        id: 'conversation-time',
        title: 'Timestamp conversation',
        updatedAt: '2026-08-12T10:14:00',
        userMessage: 'Check the timestamp',
        finalReply: 'Timestamp checked',
        sourceProject: 'Recent',
      }],
    }] })

    render(App)
    await (await screen.findByRole('button', { name: /Export conversations/ })).click()

    expect(await screen.findByText('Updated 2026-08-12 10:14')).toBeInTheDocument()
    const listTimestamps = Array.from(document.querySelectorAll('time'))
    expect(listTimestamps).toHaveLength(2)
    listTimestamps.forEach((timestamp) => expect(timestamp).toHaveTextContent('2026-08-12 10:14'))
    expect(screen.queryByText(/Aug 12|10:14 AM/)).not.toBeInTheDocument()

    await screen.getByRole('button', { name: '中文' }).click()
    expect(screen.getByText('更新时间 2026-08-12 10:14')).toBeInTheDocument()
    Array.from(document.querySelectorAll('time')).forEach((timestamp) => expect(timestamp).toHaveTextContent('2026-08-12 10:14'))
  })

  it('scans and exports Claude Code conversations through the selected source tool', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic/.codex')
    defaultClaudeRoot.mockResolvedValue('/synthetic/.claude')
    defaultClaudeSidebarRoot.mockResolvedValue('/synthetic/Claude-3p/local-agent-mode-sessions')
    scanCodexRoot.mockResolvedValue({ projects: [] })
    scanClaudeRoot.mockResolvedValue({ projects: [{
      id: 'claude-project', name: 'Claude Project', conversationCount: 1,
      conversations: [{ id: 'claude-conversation', title: 'Claude migration', updatedAt: '2026-08-15T08:00:00Z', userMessage: 'Move this Claude chat', finalReply: 'Claude export ready', sourceProject: 'Claude Project' }],
    }] })
    exportClaude.mockResolvedValue({ path: '/synthetic/Claude-conversations.zip', projectCount: 1, conversationCount: 1, bytes: 4096, integrityValid: true })

    render(App)
    await (await screen.findByRole('button', { name: /Export conversations/ })).click()
    const sourceTool = screen.getByRole('group', { name: 'Source tool' })
    await within(sourceTool).getByRole('button', { name: 'Claude Code' }).click()

    expect(await screen.findByRole('heading', { name: 'Claude migration' })).toBeInTheDocument()
    expect(screen.getByText('Claude Code reply')).toBeInTheDocument()
    await screen.getByRole('checkbox', { name: 'Select Claude migration' }).click()
    await screen.getByRole('button', { name: 'Continue export' }).click()
    await fireEvent.input(await screen.findByLabelText('Save as'), { target: { value: '/synthetic/Claude-conversations.zip' } })
    await screen.getByRole('button', { name: 'Export 1 conversation' }).click()

    expect(scanClaudeRoot).toHaveBeenCalledWith('/synthetic/Claude-3p/local-agent-mode-sessions')
    expect(exportClaude).toHaveBeenCalledWith('/synthetic/Claude-3p/local-agent-mode-sessions', {
      destination: '/synthetic/Claude-conversations.zip', conversationIds: ['claude-conversation'],
    })
  })

  it('shows a dedicated export result page instead of a toast', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic/.codex')
    scanCodexRoot.mockResolvedValue({ projects: [{
      id: 'project-result',
      name: 'Recent',
      conversationCount: 1,
      conversations: [{ id: 'conversation-result', title: 'Result conversation', updatedAt: '2026-08-14T08:00:00Z', userMessage: 'Export this', finalReply: 'Done', sourceProject: 'Recent' }],
    }] })
    exportCodex.mockResolvedValue({ path: '/Users/test/Documents/Codex Transfers/Codex-conversations-Aug-14-2026.zip', projectCount: 1, conversationCount: 1, bytes: 2_400_000, integrityValid: true })

    render(App)
    await (await screen.findByRole('button', { name: /Export conversations/ })).click()
    const checkbox = await screen.findByRole('checkbox', { name: 'Select Result conversation' })
    await checkbox.click()
    await screen.getByRole('button', { name: /Continue export/ }).click()
    const destinationInput = await screen.findByLabelText('Save as')
    await fireEvent.input(destinationInput, { target: { value: '/Users/test/Documents/Codex Transfers/Codex-conversations-Aug-14-2026.zip' } })
    await screen.getByRole('button', { name: 'Export 1 conversation' }).click()

    expect(await screen.findByRole('heading', { name: 'Export complete' })).toBeInTheDocument()
    expect(scanCodexRoot).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('group', { name: 'Language' })).toHaveClass('global-language-switch')
    expect(screen.getByText('Integrity verified')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open folder' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Export again' })).toBeInTheDocument()
    expect(screen.queryByText('Recent conversation')).not.toBeInTheDocument()
    expect(document.querySelector('.export-preview-pane')).toBeNull()
    expect(document.querySelector('.export-result-mascot')).toBeInTheDocument()
  })

  it('requires explicit loss acknowledgement before a cross-tool import can continue', async () => {
    applicationInfo.mockResolvedValue({ name: 'Codex Claude Shuttle', version: '0.1.0-dev' })
    defaultCodexRoot.mockResolvedValue('/synthetic-target/.codex')
    defaultClaudeRoot.mockResolvedValue('/synthetic-target/.claude')
    chooseImportTargetFolder.mockResolvedValue('/synthetic/project-alpha')
    inspectTransferImport.mockResolvedValue({
      bundlePath: '/synthetic/Codex-conversations.zip', sourceTool: 'codex', projectCount: 2, conversationCount: 18, message: 'ready',
      sources: [{ key: 'source-alpha', name: 'Project Alpha', sourceFolder: 'alpha', conversationCount: 18, suggestedTargetId: 'target-alpha', suggestedTargetName: 'Project Alpha' }],
      targets: [{ id: 'target-alpha', name: 'Project Alpha', folder: 'alpha' }],
    })
    prepareTransferImport.mockResolvedValue({
      planToken: 'cross-plan', sourceTool: 'codex', targetTool: 'claude', requiredBytes: 4096, availableBytes: 1_000_000,
      projectCount: 1, conversationCount: 18, createCount: 18, skipCount: 0, replaceCount: 0, toolRunning: false, canExecute: true, message: 'ready',
    })

    render(App)
    await (await screen.findByRole('button', { name: /Import conversations/ })).click()
    const pathInput = await screen.findByLabelText('Documents/codex-claude-shuttle.zip')
    await fireEvent.input(pathInput, { target: { value: '/synthetic/Codex-conversations.zip' } })
    await screen.getByRole('button', { name: 'Check file' }).click()

    const destination = await screen.findByRole('group', { name: 'Destination' })
    await within(destination).getByRole('button', { name: 'Claude Code' }).click()

    expect(await screen.findByRole('heading', { name: 'Review conversion' })).toBeInTheDocument()
    expect(screen.getByText('Source: Codex -> Target: Claude Code')).toBeInTheDocument()
    expect(screen.getByText(/You are reviewing the transfer package details and conversion scope/)).toBeInTheDocument()
    expect(screen.getByText('Tool calls and results')).toBeInTheDocument()
    expect(screen.getByText('Memory, skills, settings and attachments')).toBeInTheDocument()
    const continueButton = screen.getByRole('button', { name: 'Accept and continue' })
    expect(continueButton).toBeDisabled()

    await screen.getByRole('checkbox', { name: 'I understand these items will not be converted.' }).click()
    expect(continueButton).toBeEnabled()
    await continueButton.click()
    expect(await screen.findByText('Transfer package verified')).toBeInTheDocument()
    await screen.getByRole('button', { name: 'Choose folder' }).click()
    await (await screen.findByRole('button', { name: 'Run import preflight' })).click()
    expect(await screen.findByRole('button', { name: 'Start import' })).toBeInTheDocument()
    expect(prepareTransferImport).toHaveBeenCalledTimes(1)
    expect(prepareTransferImport.mock.calls[0][0]).toMatchObject({
      bundlePath: '/synthetic/Codex-conversations.zip', sourceTool: 'codex', targetTool: 'claude', targetRoot: '/synthetic-target/.claude',
      mappings: [{ sourceKey: 'source-alpha', targetId: '', targetDirectory: '/synthetic/project-alpha' }],
    })
  })
})
