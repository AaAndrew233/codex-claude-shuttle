<script lang="ts">
  import { onMount } from 'svelte'
  import { ArrowLeft, Check, Clock3, FileArchive, Folder, FolderOpen, House, RefreshCw, Search, ShieldCheck, Sparkles, ThumbsUp, UserRound } from '@lucide/svelte'
  import { ApplicationInfo, ChooseExportFolder, DefaultClaudeSidebarRoot, DefaultCodexRoot, ExportClaude, ExportCodex, OpenExportFolder, ScanClaudeRoot, ScanCodexRoot } from '../wailsjs/go/main/App.js'
  import ImportFlow from './components/ImportFlow.svelte'
  import LanguageSwitch from './components/LanguageSwitch.svelte'
  import TransferToolSwitch from './components/TransferToolSwitch.svelte'
  import { renderPreviewMarkdown } from './lib/markdown'

  type Conversation = { id: string; title: string; updatedAt: string; userMessage: string; finalReply: string; sourceProject: string }
  type Project = { id: string; name: string; conversationCount: number; conversations: Conversation[] }
  type HomeLanguage = 'en' | 'zh'

  const homeTranslations = {
    en: {
      actionsLabel: 'Codex Claude Shuttle actions',
      exportTitle: 'Export conversations',
      exportDescription: 'Create a transfer package from this Mac',
      importTitle: 'Import conversations',
      importDescription: 'Restore from a transfer package on this Mac',
      boundary: 'Conversations only. Project source, account, and global settings are not included.',
    },
    zh: {
      actionsLabel: '对话迁移操作',
      exportTitle: '导出对话',
      exportDescription: '从这台电脑创建迁移包',
      importTitle: '导入对话',
      importDescription: '从迁移包恢复到这台电脑',
      boundary: '只迁移对话，不包含项目源码、账号和全局设置。',
    },
  } as const

  const pageTranslations = {
    en: {
      export: {
        title: 'Export conversations', tool: 'Codex', sourceTool: 'Source tool', scanComplete: 'Scan complete', scanning: 'Scanning…', updated: 'Updated just now', reading: 'Reading visible conversations', scanAgain: 'Scan again', selectTitle: 'Select conversations', selectDescription: 'Choose the conversations to include in this transfer package.', projects: 'Projects', conversations: 'Conversations', preview: 'Preview', focusedConversation: 'Focused conversation', searchProjects: 'Search projects', searchConversations: 'Search conversations', selectAll: 'Select all in', clearSelection: 'Clear selection', focusedPreview: 'Focused preview', selectedForExport: 'Selected for export', notSelected: 'Not selected', noConversations: 'No conversations found', noMatching: 'No matching conversations', selectProject: 'Select a project to view conversations', latestExchange: 'LATEST EXCHANGE', updatedAt: 'Updated', yourMessage: 'Your message', codexReply: 'Codex reply', claudeReply: 'Claude Code reply', noConversation: 'No conversation selected', selectConversation: 'Select a conversation to preview its latest exchange.', back: 'Back', continueExport: 'Continue export', selectedSummary: (count: number, projects: number) => `${count} conversation${count === 1 ? '' : 's'} selected from ${projects} project${projects === 1 ? '' : 's'}`, boundary: 'Project source, account, and global settings are not included.', ready: 'Ready to export', review: 'Review this transfer package before saving.', conversationsLabel: 'conversations', projectsLabel: 'projects', saveAs: 'Save as', saveTo: 'Save to', chooseFolder: 'Choose folder', chooseLocalFolder: 'Choose a local folder', pathNote: 'Enter the full file path in “Save as”.', included: 'Included', history: 'Conversation history', messages: 'User messages and assistant replies', notIncluded: 'Not included', excluded: 'Project source, account, and global settings', savedLocally: 'Saved locally on this Mac', exporting: 'Exporting…', exportCount: (count: number) => `Export ${count} conversation${count === 1 ? '' : 's'}`, result: { title: 'Export complete', subtitle: 'Your transfer package is ready.', context: 'Export context', package: 'Export package', fileSize: 'File size', integrity: 'Integrity verified', openFolder: 'Open folder', exportAgain: 'Export again', backHome: 'Back to home', savedPath: 'Saved to', included: 'Included', history: 'Conversation history', messages: 'User messages and assistant replies', notIncluded: 'Not included', excluded: 'Project source, account, and global settings', openFolderError: 'Unable to open the export folder.' },
      },
      import: {
        backHome: 'Back to home', title: 'Restore conversations', description: 'Select a transfer package, inspect its contents, then choose where to write it.', progress: 'Import progress', screenTitle: 'Import conversations', screenDescription: 'Restore conversations from a verified transfer package on this Mac.', choosePackage: 'Choose a transfer package', chooseFile: 'Choose file', manualPath: 'Or enter a local file path', scopeTitle: 'Import scope', included: 'INCLUDED', history: 'Conversation history', messages: 'User messages and assistant replies', notIncluded: 'NOT INCLUDED', excluded: 'Project source and account settings', beforeImport: 'BEFORE IMPORT', packageChecked: 'The package is checked before anything is written.', package: 'Transfer package', packageDescription: 'Enter a file path available on this Mac', fileLocation: 'File location', placeholder: 'Documents/codex-claude-shuttle.zip', checkFile: 'Check file', checking: 'Checking…', importPlan: 'Import plan', viewPlan: 'View import plan', generating: 'Generating…', targetWorkspace: 'Target workspace', targetPlaceholder: 'Documents/codex-claude-shuttle-target', closed: 'I have closed the target tool and allow this import', runPreflight: 'Run import preflight', preflighting: 'Running preflight…', startImport: 'Start import', importing: 'Importing…', complete: 'Import complete', invalidSource: 'Integrity validation only confirms that the file is intact; it does not establish trust.', projects: 'projects', conversations: 'conversations', available: 'available', required: 'required', write: 'Written', skipped: 'Skipped', replaced: 'Replaced', canImport: 'Ready to import', needsTarget: 'Target selection required', createProject: 'Create an empty project', importAction: 'Import', chooseTarget: 'Choose target', continue: 'Continue',
      },
    },
    zh: {
      export: {
        title: '导出对话', tool: 'Codex', sourceTool: '来源工具', scanComplete: '扫描完成', scanning: '正在扫描…', updated: '刚刚更新', reading: '正在读取可见对话', scanAgain: '重新扫描', selectTitle: '选择对话', selectDescription: '选择要包含在迁移包中的对话。', projects: '项目', conversations: '对话', preview: '预览', focusedConversation: '当前对话', searchProjects: '搜索项目', searchConversations: '搜索对话', selectAll: '选择全部', clearSelection: '清除选择', focusedPreview: '当前预览', selectedForExport: '已选择导出', notSelected: '未选择', noConversations: '没有找到对话', noMatching: '没有匹配的对话', selectProject: '选择项目后查看对话', latestExchange: '最近一轮对话', updatedAt: '更新时间', yourMessage: '你的消息', codexReply: 'Codex 回复', claudeReply: 'Claude Code 回复', noConversation: '未选择对话', selectConversation: '选择一个对话，查看最近一轮消息。', back: '返回', continueExport: '继续导出', selectedSummary: (count: number, projects: number) => `已选择 ${count} 个对话，来自 ${projects} 个项目`, boundary: '不包含项目源码、账号和全局设置。', ready: '准备导出', review: '保存前请核对这个迁移包。', conversationsLabel: '个对话', projectsLabel: '个项目', saveAs: '保存为', saveTo: '保存到', chooseFolder: '选择文件夹', chooseLocalFolder: '选择本地文件夹', pathNote: '请在“保存为”中输入完整文件路径。', included: '包含内容', history: '对话历史', messages: '用户消息和助手回复', notIncluded: '不包含', excluded: '项目源码、账号和全局设置', savedLocally: '保存在这台 Mac 本地', exporting: '正在导出…', exportCount: (count: number) => `导出 ${count} 个对话`, result: { title: '导出完成', subtitle: '迁移包已准备好。', context: '导出摘要', package: '迁移包', fileSize: '文件大小', integrity: '完整性校验通过', openFolder: '打开文件夹', exportAgain: '再次导出', backHome: '返回首页', savedPath: '保存位置', included: '包含内容', history: '对话历史', messages: '用户消息和助手回复', notIncluded: '不包含', excluded: '项目源码、账号和全局设置', openFolderError: '无法打开迁移包所在文件夹。' },
      },
      import: {
        backHome: '返回首页', title: '恢复对话', description: '选择迁移包，检查内容后再决定写入位置。', progress: '导入进度', screenTitle: '导入对话', screenDescription: '从这台 Mac 上经过检查的迁移包恢复对话。', choosePackage: '选择迁移包', chooseFile: '选择文件', manualPath: '或输入本地文件路径', scopeTitle: '导入范围', included: '包含内容', history: '对话历史', messages: '用户消息和助手回复', notIncluded: '不包含', excluded: '项目源码和账号设置', beforeImport: '导入前', packageChecked: '写入任何内容前都会先检查迁移包。', package: '迁移包', packageDescription: '输入本机可访问的文件路径', fileLocation: '文件位置', placeholder: '文稿/Codex-Claude-Shuttle.zip', checkFile: '检查文件', checking: '检查中…', importPlan: '导入计划', viewPlan: '查看导入计划', generating: '生成中…', targetWorkspace: '目标工作区', targetPlaceholder: '文稿/Codex-Claude-Shuttle-目标', closed: '我已关闭目标工具，允许执行本次导入', runPreflight: '运行导入预检', preflighting: '预检中…', startImport: '开始导入', importing: '导入中…', complete: '导入完成', invalidSource: '完整性校验只表示文件未损坏，不代表来源可信。', projects: '个项目', conversations: '个对话', available: '可用', required: '需要', write: '写入', skipped: '跳过', replaced: '替换', canImport: '可以导入', needsTarget: '需要选择目标', createProject: '建议创建空项目', importAction: '导入', chooseTarget: '选择目标', continue: '继续',
      },
    },
  } as const

  let appName = 'Codex Claude Shuttle'
  let status: 'loading' | 'ready' | 'error' = 'loading'
  let mode: 'home' | 'export' | 'import' = 'home'
  let language: HomeLanguage = 'en'
  let projects: Project[] = []
  let sourceRoot = ''
  let selectedProjectId = ''
  let selectedConversationIds: string[] = []
  let selectedConversation: Conversation | null = null
  let destination = ''
  let exportResult: { path: string; projectCount: number; conversationCount: number; bytes: number; integrityValid: boolean } | null = null
  let showDestination = false
  let errorMessage = ''
  let busy = false
  let scanning = false
  let projectQuery = ''
  let conversationQuery = ''
  let exportSourceTool: 'codex' | 'claude' = 'codex'

  $: destinationFolder = destination.trim().replace(/[\\/][^\\/]*$/, '') || pageCopy.export.chooseLocalFolder
  $: destinationFileName = destination.trim().split(/[\\/]/).pop() || ''
  $: resultFileName = exportResult ? destinationFileName || fileNameFromPath(exportResult.path) : ''
  $: resultFolder = exportResult ? destinationFolder || folderFromPath(exportResult.path) : ''

  $: selectedProject = projects.find((project) => project.id === selectedProjectId) ?? null
  $: selectedCount = selectedConversationIds.length
  $: selectedProjectCount = projects.filter((project) => project.conversations.some((conversation) => selectedConversationIds.includes(conversation.id))).length
  $: visibleProjects = projects.filter((project) => project.name.toLowerCase().includes(projectQuery.trim().toLowerCase()))
  $: visibleConversations = selectedProject?.conversations.filter((conversation) => conversation.title.toLowerCase().includes(conversationQuery.trim().toLowerCase())) ?? []
  $: allVisibleSelected = visibleConversations.length > 0 && visibleConversations.every((conversation) => selectedConversationIds.includes(conversation.id))
  $: homeCopy = homeTranslations[language]
  $: pageCopy = pageTranslations[language]
  $: renderedUserMessage = renderPreviewMarkdown(selectedConversation?.userMessage || 'No user message available.')
  $: renderedFinalReply = renderPreviewMarkdown(selectedConversation?.finalReply || 'No assistant reply available.')

  onMount(async () => {
    try {
      const info = await ApplicationInfo()
      appName = info.name
      status = 'ready'
    } catch {
      if (import.meta.env.DEV && import.meta.env.MODE !== 'test') {
        status = 'ready'
        return
      }
      status = 'error'
      errorMessage = '桌面服务暂时不可用，请重新启动应用。'
    }
  })

  async function startExport() {
    mode = 'export'; exportSourceTool = 'codex'; projects = []; selectedConversation = null; selectedConversationIds = []; errorMessage = ''
    try { sourceRoot = await DefaultCodexRoot() } catch (error) { errorMessage = error instanceof Error ? error.message : '无法确定 Codex 数据目录。'; return }
    await scan()
  }

  function startImport() { mode = 'import'; errorMessage = '' }

  async function scan() {
    busy = true; scanning = true; errorMessage = ''
    try {
      const result = exportSourceTool === 'codex' ? await ScanCodexRoot(sourceRoot.trim()) : await ScanClaudeRoot(sourceRoot.trim())
      projects = result.projects
      selectedProjectId = result.projects[0]?.id ?? ''; selectedConversationIds = []; selectedConversation = result.projects[0]?.conversations[0] ?? null
    } catch (error) { errorMessage = error instanceof Error ? error.message : '扫描失败，请重试。' } finally { scanning = false; busy = false }
  }

  async function changeExportSource(event: CustomEvent<'codex' | 'claude'>) {
    exportSourceTool = event.detail
    projects = []
    selectedProjectId = ''
    selectedConversationIds = []
    selectedConversation = null
    projectQuery = ''
    conversationQuery = ''
    errorMessage = ''
    try {
      sourceRoot = exportSourceTool === 'codex' ? await DefaultCodexRoot() : await DefaultClaudeSidebarRoot()
      await scan()
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '无法确定来源工具数据目录。'
    }
  }

  function toggleConversation(id: string) { selectedConversationIds = selectedConversationIds.includes(id) ? selectedConversationIds.filter((item) => item !== id) : [...selectedConversationIds, id] }
  function toggleVisibleConversations() { const ids = visibleConversations.map((conversation) => conversation.id); selectedConversationIds = allVisibleSelected ? selectedConversationIds.filter((id) => !ids.includes(id)) : [...new Set([...selectedConversationIds, ...ids])] }
  function focusConversation(conversation: Conversation | null) { selectedConversation = conversation }
  function focusProject(project: Project) { selectedProjectId = project.id; conversationQuery = ''; focusConversation(project.conversations[0] ?? null) }
  function displayProjectName(project: Project | null) { return project?.name === '最近' ? 'Recent' : project?.name ?? 'project' }
  function handleConversationKeydown(event: KeyboardEvent, conversation: Conversation) {
    if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); focusConversation(conversation) }
  }

  async function exportBundle() {
    if (!destination.trim() || selectedCount === 0) return
    busy = true; errorMessage = ''
    try {
      const request = { destination: destination.trim(), conversationIds: selectedConversationIds }
      exportResult = exportSourceTool === 'codex' ? await ExportCodex(sourceRoot.trim(), request) : await ExportClaude(sourceRoot.trim(), request)
      showDestination = false
    } catch (error) { errorMessage = error instanceof Error ? error.message : '导出失败，请重试。' } finally { busy = false }
  }

  async function openExportFolder() {
    if (!exportResult) return
    errorMessage = ''
    try { await OpenExportFolder(exportResult.path) } catch (error) { errorMessage = error instanceof Error ? error.message : pageCopy.export.result.openFolderError }
  }

  function exportAgain() { exportResult = null; errorMessage = ''; showDestination = false }

  function backHome() { mode = 'home'; errorMessage = ''; exportResult = null; showDestination = false }
  function openExportConfirmation() {
    if (scanning || selectedCount === 0) return
    showDestination = true
    errorMessage = ''
  }
  async function chooseExportFolder() {
    busy = true
    errorMessage = ''
    try {
      const currentFolder = destination.trim().replace(/[\\/][^\\/]*$/, '')
      const selectedFolder = await ChooseExportFolder(currentFolder)
      if (selectedFolder) {
        const fileName = destination.trim().split(/[\\/]/).pop() || `${exportSourceTool === 'codex' ? 'Codex' : 'Claude'}-conversations.zip`
        destination = `${selectedFolder.replace(/[\\/]+$/, '')}/${fileName}`
      }
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '无法打开系统文件夹选择器。'
    } finally {
      busy = false
    }
  }
  function formatDate(value: string) { return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
  function formatExportDate(value: string) {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return ''
    const pad = (part: number) => String(part).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
  }
  function formatBytes(bytes: number) { return bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB` }
  function fileNameFromPath(path: string) { return path.split(/[\\/]/).pop() || path }
  function folderFromPath(path: string) { return path.replace(/[\\/][^\\/]*$/, '') || path }
</script>

{#if status === 'loading'}
  <main class="startup-shell" aria-live="polite"><span class="section-kicker">CT</span><h1>准备迁移工作区</h1><p>正在连接本地服务…</p><div class="progress"></div></main>
{:else if status === 'error'}
  <main class="startup-shell" aria-live="assertive"><span class="section-kicker">CT</span><h1>无法启动</h1><p>{errorMessage}</p></main>
{:else}
  <div class="app-shell" class:export-mode={mode === 'export'}>
    {#if mode === 'home'}
      <main class="mascot-home">
        <aside class="home-brand-panel" aria-labelledby="home-title">
          <div class="home-brand-lockup">
            <img class="home-brand-mark" src="/assets/home/transfer-brand-mark-v2.png" alt="" />
            <h1 id="home-title" aria-label="Codex Claude Shuttle">Codex<br />Claude<br />Shuttle</h1>
          </div>
          <img class="home-mascot" src="/assets/home/mole-courier-v2.png" alt="小鼹鼠搬运员抱着迁移纸箱" />
        </aside>

        <section class="home-entry-panel" aria-label={homeCopy.actionsLabel} lang={language === 'zh' ? 'zh-CN' : 'en'}>
          <LanguageSwitch bind:language className="home-language-switch" />

          <div class="home-entry-actions">
            <button class="home-entry-action" on:click={startExport}>
              <img class="home-file-icon" src="/assets/home/export-conversations.png" alt="" />
              <span class="home-entry-copy">
                <strong>{homeCopy.exportTitle}</strong>
                <small>{homeCopy.exportDescription}</small>
              </span>
              <img class="home-entry-chevron" src="/assets/home/entry-arrow.png" alt="" />
            </button>

            <button class="home-entry-action" on:click={startImport}>
              <img class="home-file-icon" src="/assets/home/import-conversations.png" alt="" />
              <span class="home-entry-copy">
                <strong>{homeCopy.importTitle}</strong>
                <small>{homeCopy.importDescription}</small>
              </span>
              <img class="home-entry-chevron" src="/assets/home/entry-arrow.png" alt="" />
            </button>
          </div>

          <p class="home-entry-boundary">
            <img class="home-boundary-check" src="/assets/home/conversations-only-check.png" alt="" />
            {homeCopy.boundary}
          </p>
        </section>
      </main>
    {:else if mode === 'export'}
      <main class="export-v5" class:export-result-mode={Boolean(exportResult)}>
        {#if exportResult}
          <header class="export-result-taskbar" lang={language === 'zh' ? 'zh-CN' : 'en'}>
            <div class="export-result-brand"><span class="export-result-brand-mark" aria-hidden="true">C</span><strong>{appName}</strong></div>
            <div class="export-result-task-actions">
              <LanguageSwitch bind:language />
              <div class="export-scan-status"><strong>{pageCopy.export.scanComplete}</strong><small>{pageCopy.export.updated}</small></div>
              <button class="export-scan-button" disabled={busy} on:click={scan}><RefreshCw size={18} strokeWidth={2.1} />{scanning ? pageCopy.export.scanning : pageCopy.export.scanAgain}</button>
            </div>
          </header>
          <section class="export-result-layout" aria-labelledby="export-result-title">
            <aside class="export-result-context">
              <h2>{pageCopy.export.result.context}</h2>
              <div class="export-result-context-list">
                <div><FolderOpen size={24} strokeWidth={1.8} /><span>{resultFolder}</span></div>
                <div><span class="export-result-context-number">{exportResult.conversationCount}</span><span>{pageCopy.export.conversations}</span></div>
                <div><span class="export-result-context-number">{exportResult.projectCount}</span><span>{pageCopy.export.projects}</span></div>
                <div><FileArchive size={24} strokeWidth={1.8} /><span>{formatBytes(exportResult.bytes)}</span></div>
                <div class="verified"><ShieldCheck size={24} strokeWidth={1.8} /><span>{pageCopy.export.result.integrity}</span></div>
              </div>
            </aside>
            <section class="export-result-main">
              <div class="export-result-heading">
                <span class="export-result-success-mark" aria-hidden="true"><Check size={34} strokeWidth={2.4} /></span>
                <h1 id="export-result-title">{pageCopy.export.result.title}</h1>
                <p>{pageCopy.export.result.subtitle}</p>
              </div>
              {#if errorMessage}<div class="inline-notice error export-result-error" role="alert"><span class="status-icon">!</span>{errorMessage}</div>{/if}
              <div class="export-result-package">
                <h2>{pageCopy.export.result.package}</h2>
                <div class="export-result-file"><FileArchive size={48} strokeWidth={1.5} /><div><strong>{resultFileName}</strong><small>{formatBytes(exportResult.bytes)}</small></div></div>
                <button type="button" class="export-result-folder-link" on:click={openExportFolder}><FolderOpen size={22} strokeWidth={1.8} />{pageCopy.export.result.openFolder}</button>
              </div>
              <div class="export-result-scope">
                <div><h2>{pageCopy.export.result.included}</h2><p><Check size={19} strokeWidth={2.4} />{pageCopy.export.result.history}</p><p><Check size={19} strokeWidth={2.4} />{pageCopy.export.result.messages}</p></div>
                <div><h2>{pageCopy.export.result.notIncluded}</h2><p class="excluded"><span aria-hidden="true">×</span>{pageCopy.export.result.excluded}</p></div>
              </div>
              <img class="export-result-mascot" src="/assets/export/export-mole-complete.png" alt={language === 'zh' ? '小鼹鼠为完整性校验点赞' : 'Mole mascot giving a thumbs-up for the verified export'} />
              <footer class="export-result-actions"><button type="button" class="export-result-secondary" on:click={backHome}><House size={19} strokeWidth={1.9} />{pageCopy.export.result.backHome}</button><button type="button" class="export-result-primary" on:click={exportAgain}><RefreshCw size={19} strokeWidth={1.9} />{pageCopy.export.result.exportAgain}</button></footer>
            </section>
          </section>
        {:else}
        <header class="export-taskbar" lang={language === 'zh' ? 'zh-CN' : 'en'}>
          <div class="export-task-identity"><button class="export-icon-button" on:click={backHome} aria-label={language === 'zh' ? '返回首页' : 'Back to home'}><ArrowLeft size={26} strokeWidth={2.1} /></button><div class="export-task-title"><strong>{appName}</strong></div></div>
          <TransferToolSwitch bind:value={exportSourceTool} ariaLabel={pageCopy.export.sourceTool} on:change={changeExportSource} />
          <div class="export-scan-area"><LanguageSwitch bind:language /><div class="export-scan-status"><strong>{scanning ? pageCopy.export.scanning : pageCopy.export.scanComplete}</strong><small>{scanning ? pageCopy.export.reading : pageCopy.export.updated}</small></div><button class="export-scan-button" disabled={!sourceRoot.trim() || busy} on:click={scan}><RefreshCw size={18} strokeWidth={2.1} />{scanning ? pageCopy.export.scanning : pageCopy.export.scanAgain}</button></div>
        </header>
        <section class="export-page-heading"><div><h1>{pageCopy.export.selectTitle}</h1><p>{pageCopy.export.selectDescription}</p></div></section>
        {#if scanning}<div class="export-scan-progress" role="progressbar" aria-label={`Reading ${exportSourceTool === 'codex' ? 'Codex' : 'Claude Code'} conversations`}><span></span><p>{pageCopy.export.reading}…</p></div>{/if}
        {#if errorMessage}<div class="inline-notice error export-notice" role="alert"><span class="status-icon">!</span>{errorMessage}</div>{/if}
        <section class="export-workspace-grid">
          <aside class="export-pane export-project-pane">
            <div class="export-pane-header"><h2>{pageCopy.export.projects}</h2><span>{projects.length}</span></div>
            <label class="export-search" aria-label={pageCopy.export.searchProjects}><Search size={18} /><input bind:value={projectQuery} placeholder={pageCopy.export.searchProjects} /></label>
            <div class="export-project-list">{#if scanning}<p class="export-pane-empty">{pageCopy.export.scanning}</p>{:else if visibleProjects.length === 0}<p class="export-pane-empty">{pageCopy.export.noConversations}</p>{:else}{#each visibleProjects as project}<button class:focused={project.id === selectedProjectId} class="export-project-row" on:click={() => focusProject(project)}><span class="export-project-icon">{#if project.id === 'recent' || project.name === 'Recent' || project.name === '最近'}<Clock3 size={25} />{:else}<Folder size={25} />{/if}</span><span><strong>{displayProjectName(project)}</strong><small>{project.conversationCount} {pageCopy.export.conversations}</small></span><time>{project.conversations[0] ? formatExportDate(project.conversations[0].updatedAt) : ''}</time></button>{/each}{/if}</div>
          </aside>
          <section class="export-pane export-conversation-pane">
            <div class="export-pane-header"><h2>{pageCopy.export.conversations}</h2><span>{visibleConversations.length}</span></div>
            <label class="export-search" aria-label={pageCopy.export.searchConversations}><Search size={18} /><input bind:value={conversationQuery} placeholder={pageCopy.export.searchConversations} /></label>
            <div class="export-conversation-tools"><label class="export-select-all"><input type="checkbox" checked={allVisibleSelected} disabled={!visibleConversations.length} on:change={toggleVisibleConversations} /><span class="export-check-box"></span><span>{pageCopy.export.selectAll} {displayProjectName(selectedProject)}</span></label><button class="export-clear-button" disabled={!selectedCount} on:click={() => { selectedConversationIds = [] }}>{pageCopy.export.clearSelection}</button></div>
            <div class="export-conversation-list">{#if visibleConversations.length}{#each visibleConversations as conversation}<div class:focused={selectedConversation?.id === conversation.id} class="export-conversation-row" role="button" tabindex="0" on:click={() => focusConversation(conversation)} on:keydown={(event) => handleConversationKeydown(event, conversation)}><label class="export-conversation-check" aria-label={`Select ${conversation.title}`}><input type="checkbox" checked={selectedConversationIds.includes(conversation.id)} on:click|stopPropagation on:change={() => toggleConversation(conversation.id)} /><span class="export-check-box"></span></label><span class="export-conversation-copy"><strong>{conversation.title}</strong><small>{selectedConversation?.id === conversation.id ? pageCopy.export.focusedPreview : selectedConversationIds.includes(conversation.id) ? pageCopy.export.selectedForExport : pageCopy.export.notSelected}</small></span><time>{formatExportDate(conversation.updatedAt)}</time></div>{/each}{:else}<p class="export-pane-empty">{selectedProject ? pageCopy.export.noMatching : pageCopy.export.selectProject}</p>{/if}</div>
          </section>
          <aside class="export-pane export-preview-pane">
            <div class="export-pane-header"><h2>{pageCopy.export.preview}</h2><div class="export-preview-heading-meta"><span>{pageCopy.export.focusedConversation}</span><img class="export-preview-mascot" src="/assets/export/export-mole-courier.png" alt="" aria-hidden="true" /></div></div>
            {#if selectedConversation}<div class="export-preview-body"><span class="export-preview-kicker">{pageCopy.export.latestExchange}</span><div class="export-preview-title" role="heading" aria-level="3">{selectedConversation.title || selectedConversation.userMessage.slice(0, 72)}</div><span class="export-preview-time">{pageCopy.export.updatedAt} {formatExportDate(selectedConversation.updatedAt)}</span><!-- svelte-ignore a11y_no_noninteractive_tabindex (scrollable region must remain keyboard reachable) --><div class="export-message-exchange" role="region" aria-label="Latest conversation messages" tabindex="0"><div class="export-message export-user-message"><span class="export-message-label"><UserRound size={16} />{pageCopy.export.yourMessage}</span><div class="export-message-content">{@html renderedUserMessage}</div></div><div class="export-message export-assistant-message"><span class="export-message-label"><Sparkles size={16} />{exportSourceTool === 'codex' ? pageCopy.export.codexReply : pageCopy.export.claudeReply}</span><div class="export-message-content">{@html renderedFinalReply}</div></div></div></div>{:else}<div class="export-inspector-empty"><strong>{pageCopy.export.noConversation}</strong><p>{pageCopy.export.selectConversation}</p></div>{/if}
          </aside>
        </section>
        <footer class="export-actionbar"><div class="export-selection-summary"><span class="export-summary-count">{selectedCount}</span><span><strong>{pageCopy.export.selectedSummary(selectedCount, selectedProjectCount)}</strong><small>{pageCopy.export.boundary}</small></span></div><button class="export-footer-button" on:click={backHome}>{pageCopy.export.back}</button><button class="export-footer-button primary" disabled={selectedCount === 0 || scanning} on:click={openExportConfirmation}>{pageCopy.export.continueExport}</button></footer>
        {#if selectedCount > 0 && showDestination}
          <div class="export-confirm-backdrop" role="presentation" on:click={(event) => { if (event.target === event.currentTarget) showDestination = false }}>
            <div class="export-confirm-modal" role="dialog" aria-modal="true" aria-labelledby="export-confirm-title">
              <div class="export-confirm-scroll">
                <div class="export-confirm-heading">
                  <div>
                    <h2 id="export-confirm-title">{pageCopy.export.ready}</h2>
                    <p>{pageCopy.export.review}</p>
                  </div>
                  <img class="export-confirm-mascot" src="/assets/export/export-mole-courier.png" alt="小鼹鼠正在核对迁移包" />
                </div>
                <div class="export-confirm-stats" aria-label="Transfer package summary">
                  <div><strong>{selectedCount}</strong><span>{pageCopy.export.conversationsLabel}</span></div>
                  <div><strong>{selectedProjectCount}</strong><span>{pageCopy.export.projectsLabel}</span></div>
                </div>
                <div class="export-confirm-field">
                  <label for="destination">{pageCopy.export.saveAs}</label>
                  <input id="destination" bind:value={destination} placeholder="Documents/conversations.zip" />
                </div>
                <div class="export-confirm-field">
                  <label for="destination-folder">{pageCopy.export.saveTo}</label>
                  <div class="export-confirm-folder-row">
                    <input id="destination-folder" value={destinationFolder} readonly aria-describedby="destination-folder-note" />
                    <button type="button" class="export-confirm-folder-button" disabled={busy} on:click={chooseExportFolder}>{pageCopy.export.chooseFolder}</button>
                  </div>
                  <small id="destination-folder-note">{pageCopy.export.pathNote}</small>
                </div>
                <div class="export-confirm-includes">
                  <h3>{pageCopy.export.included}</h3>
                  <p><Check size={17} strokeWidth={2.4} />{pageCopy.export.history}</p>
                  <p><Check size={17} strokeWidth={2.4} />{pageCopy.export.messages}</p>
                  <h3>{pageCopy.export.notIncluded}</h3>
                  <p class="excluded"><span aria-hidden="true">×</span>{pageCopy.export.excluded}</p>
                </div>
                <p class="export-confirm-local-note">{pageCopy.export.savedLocally}</p>
              </div>
              <footer class="export-confirm-actions">
                <button type="button" class="export-confirm-back" on:click={() => { showDestination = false }}>{pageCopy.export.back}</button>
                <button type="button" class="export-confirm-submit" disabled={!destination.trim() || busy} on:click={exportBundle}>{busy ? pageCopy.export.exporting : pageCopy.export.exportCount(selectedCount)}</button>
              </footer>
            </div>
          </div>
        {/if}
        {/if}
      </main>
    {:else}
      <ImportFlow bind:language on:back={backHome} />
    {/if}
  </div>
{/if}
