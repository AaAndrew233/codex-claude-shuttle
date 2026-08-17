<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte'
  import { ArrowLeft, Check, CircleCheck, FileArchive, FolderOpen, Minus, RefreshCw, ShieldCheck } from '@lucide/svelte'
  import {
    ChooseImportFile,
    ChooseImportTargetFolder,
    DefaultClaudeRoot,
    DefaultCodexRoot,
    ExecuteTransferImport,
    InspectTransferImport,
    PrepareTransferImport,
  } from '../../wailsjs/go/main/App.js'
  import { domain } from '../../wailsjs/go/models'
  import ImportReview from './ImportReview.svelte'
  import LanguageSwitch from './LanguageSwitch.svelte'

  export let language: 'en' | 'zh' = 'en'

  const dispatch = createEventDispatcher<{ back: void }>()
  const copy = {
    en: {
      progress: 'Import progress', steps: ['Choose package', 'Review contents', 'Resolve conflicts', 'Preflight', 'Complete'],
      title: 'Import conversations', description: 'Restore or convert conversations from a transfer package on this Mac.',
      choosePackage: 'Choose a transfer package', chooseFile: 'Choose file', manualPath: 'Or enter a local file path', placeholder: 'Documents/codex-claude-shuttle.zip', checkFile: 'Check file', checking: 'Checking…',
      scopeTitle: 'Import scope', included: 'INCLUDED', history: 'Conversation history', messages: 'User messages and assistant replies', notIncluded: 'NOT INCLUDED', excluded: 'Project source and account settings', beforeImport: 'BEFORE IMPORT', packageChecked: 'The package is checked before anything is written.',
      packageReady: 'Transfer package verified', sourceWarning: 'Integrity validation confirms the file is intact, not that its source is trusted.', projects: 'projects', conversations: 'conversations',
      mappingTitle: 'Choose target projects', mappingDescription: 'Map each source project to an existing folder on this Mac.', sourceFolder: 'Source folder', chooseTarget: 'Choose a target project', anotherFolder: 'Another folder…', chooseFolder: 'Choose folder', selectedFolder: 'Selected folder', mappingMissing: 'Choose a target for every source project.',
      conflictTitle: 'When a conversation already exists', skip: 'Keep the conversation already on this Mac', replace: 'Replace it with the conversation from this package', replaceWarning: 'Replaced files are backed up and restored automatically if the import fails.',
      runPreflight: 'Run import preflight', preflighting: 'Running preflight…', retryPreflight: 'Check again', toolRunning: (tool: string) => `${tool} is still running. Quit ${tool} completely, then check again.`,
      required: 'required', available: 'available', create: 'New', skipped: 'Skipped', replaced: 'Replaced', startImport: 'Start import', importing: 'Importing…',
      backHome: 'Back to home', continue: 'Continue', resultTitle: 'Import complete', pendingTitle: 'Conversations imported', resultSubtitle: 'The conversation files passed local verification.', pendingSubtitle: (tool: string) => `The files passed local verification, but ${tool} has not discovered every conversation yet.`,
      filesWritten: 'Files written', filesVerified: 'Files verified', discoveredBy: (tool: string) => `Discovered by ${tool}`, pendingDiscovery: 'Awaiting discovery', warnings: 'Needs attention', importAnother: 'Import another package',
      back: 'Back', acceptConversion: 'Accept and continue',
    },
    zh: {
      progress: '导入进度', steps: ['选择迁移包', '检查内容', '处理冲突', '导入预检', '完成'],
      title: '导入对话', description: '从这台电脑的迁移包中恢复或转换对话。',
      choosePackage: '选择迁移包', chooseFile: '选择文件', manualPath: '或输入本地文件路径', placeholder: '文稿/Codex-Claude-Shuttle.zip', checkFile: '检查文件', checking: '检查中…',
      scopeTitle: '导入范围', included: '包含内容', history: '对话历史', messages: '用户消息和助手回复', notIncluded: '不包含', excluded: '项目源码和账号设置', beforeImport: '导入前', packageChecked: '写入任何内容前都会先检查迁移包。',
      packageReady: '迁移包检查通过', sourceWarning: '完整性校验只表示文件未损坏，不代表来源可信。', projects: '个项目', conversations: '个对话',
      mappingTitle: '选择目标项目', mappingDescription: '为每个来源项目选择这台电脑上的现有文件夹。', sourceFolder: '来源文件夹', chooseTarget: '选择目标项目', anotherFolder: '其他文件夹…', chooseFolder: '选择文件夹', selectedFolder: '已选文件夹', mappingMissing: '请为每个来源项目选择目标位置。',
      conflictTitle: '遇到已存在的对话时', skip: '保留这台电脑上的现有对话', replace: '使用迁移包中的对话替换', replaceWarning: '替换前会创建备份；导入失败时自动恢复。',
      runPreflight: '运行导入预检', preflighting: '预检中…', retryPreflight: '重新检查', toolRunning: (tool: string) => `${tool} 仍在运行。请完全退出 ${tool} 后重新检查。`,
      required: '需要', available: '可用', create: '新增', skipped: '跳过', replaced: '替换', startImport: '开始导入', importing: '正在导入…',
      backHome: '返回首页', continue: '继续', resultTitle: '导入完成', pendingTitle: '对话文件已导入', resultSubtitle: '对话文件已通过本地写入校验。', pendingSubtitle: (tool: string) => `文件已通过本地写入校验，但 ${tool} 尚未发现全部对话。`,
      filesWritten: '写入文件', filesVerified: '文件校验', discoveredBy: (tool: string) => `${tool} 已发现`, pendingDiscovery: '等待发现', warnings: '需要留意', importAnother: '继续导入其他迁移包',
      back: '返回', acceptConversion: '接受并继续',
    },
  } as const

  type MappingChoice = { targetId: string; targetDirectory: string }

  let targetRoots: Record<'codex' | 'claude', string> = { codex: '', claude: '' }
  let bundlePath = ''
  let inspection: domain.TransferImportInspection | null = null
  let mappings: Record<string, MappingChoice> = {}
  let conflictPolicy: 'skip' | 'replace' = 'skip'
  let preflight: domain.TransferImportPreflight | null = null
  let execution: domain.TransferImportExecutionResult | null = null
  let errorMessage = ''
  let busy = false
  let targetTool: 'codex' | 'claude' = 'codex'
  let sourceTool: 'codex' | 'claude' = 'codex'
  let reviewAccepted = false
  let conversionAcknowledged = false

  $: text = copy[language]
  $: allMappingsReady = Boolean(inspection?.sources.length) && inspection!.sources.every((source) => {
    const mapping = mappings[source.key]
    return Boolean(mapping?.targetId || mapping?.targetDirectory)
  })
  $: importStep = execution ? 5 : preflight ? 4 : inspection && reviewAccepted ? 3 : inspection ? 2 : 1
  $: isCrossTool = targetTool !== sourceTool
  $: targetToolName = targetTool === 'codex' ? 'Codex' : 'Claude Code'

  onMount(async () => {
    try {
      const [codexRoot, claudeRoot] = await Promise.all([DefaultCodexRoot(), DefaultClaudeRoot()])
      targetRoots = { codex: codexRoot, claude: claudeRoot }
    } catch (error) {
      errorMessage = messageFrom(error, language === 'zh' ? '无法确定工具数据目录。' : 'Unable to locate the tool data folders.')
    }
  })

  function messageFrom(error: unknown, fallback: string) {
    return error instanceof Error ? error.message : fallback
  }

  function clearInspection() {
    inspection = null
    mappings = {}
    preflight = null
    execution = null
    targetTool = 'codex'
    sourceTool = 'codex'
    reviewAccepted = false
    conversionAcknowledged = false
    errorMessage = ''
  }

  async function inspectBundle() {
    if (!bundlePath.trim()) return
    busy = true
    errorMessage = ''
    try {
      const result = await InspectTransferImport(bundlePath.trim(), '', '')
      inspection = result
      sourceTool = result.sourceTool as 'codex' | 'claude'
      targetTool = sourceTool
      reviewAccepted = false
      conversionAcknowledged = false
      mappings = Object.fromEntries(result.sources.map((source) => [source.key, {
        targetId: source.suggestedTargetId || '',
        targetDirectory: '',
      }]))
      preflight = null
      execution = null
    } catch (error) {
      inspection = null
      mappings = {}
      errorMessage = messageFrom(error, language === 'zh' ? '无法读取迁移包。' : 'Unable to read the transfer package.')
    } finally {
      busy = false
    }
  }

  async function acceptReview() {
    if (!inspection) return
    if (isCrossTool && !conversionAcknowledged) return
    busy = true
    errorMessage = ''
    try {
      const root = await ensureTargetRoot(targetTool)
      const refreshed = await InspectTransferImport(inspection.bundlePath, targetTool, root)
      inspection = refreshed
      sourceTool = refreshed.sourceTool as 'codex' | 'claude'
      mappings = Object.fromEntries(refreshed.sources.map((source) => [source.key, {
        targetId: source.suggestedTargetId || '',
        targetDirectory: '',
      }]))
      reviewAccepted = true
    } catch (error) {
      errorMessage = messageFrom(error, language === 'zh' ? '无法读取目标工具项目。' : 'Unable to read target tool projects.')
    } finally {
      busy = false
    }
  }

  function returnToPackage() {
    clearInspection()
    bundlePath = ''
  }

  async function chooseImportFile() {
    busy = true
    errorMessage = ''
    try {
      const selectedFile = await ChooseImportFile(folderFromPath(bundlePath))
      if (!selectedFile) return
      bundlePath = selectedFile
      clearInspection()
    } catch (error) {
      errorMessage = messageFrom(error, language === 'zh' ? '无法打开系统文件选择器。' : 'Unable to open the file picker.')
      busy = false
      return
    }
    busy = false
    await inspectBundle()
  }

  function selectTarget(sourceKey: string, event: Event) {
    const targetId = (event.currentTarget as HTMLSelectElement).value
    if (targetId === '__custom__') return
    mappings = { ...mappings, [sourceKey]: { targetId, targetDirectory: '' } }
    preflight = null
  }

  async function chooseTargetFolder(sourceKey: string) {
    busy = true
    errorMessage = ''
    try {
      const current = mappings[sourceKey]?.targetDirectory || ''
      const selected = await ChooseImportTargetFolder(current)
      if (selected) {
        mappings = { ...mappings, [sourceKey]: { targetId: '', targetDirectory: selected } }
        preflight = null
      }
    } catch (error) {
      errorMessage = messageFrom(error, language === 'zh' ? '无法打开目标文件夹选择器。' : 'Unable to open the target folder picker.')
    } finally {
      busy = false
    }
  }

  async function prepareImport() {
    if (!inspection || !allMappingsReady) return
    busy = true
    errorMessage = ''
    try {
      const targetRoot = await ensureTargetRoot(targetTool)
      preflight = await PrepareTransferImport(new domain.TransferImportRequest({
        bundlePath: inspection.bundlePath,
        sourceTool,
        targetTool,
        targetRoot,
        mappings: inspection.sources.map((source) => ({ sourceKey: source.key, ...mappings[source.key] })),
        conflictPolicy,
      }))
      execution = null
    } catch (error) {
      preflight = null
      errorMessage = messageFrom(error, language === 'zh' ? '导入预检失败。' : 'Import preflight failed.')
    } finally {
      busy = false
    }
  }

  async function executeImport() {
    if (!preflight?.canExecute || !preflight.planToken) return
    busy = true
    errorMessage = ''
    try {
      execution = await ExecuteTransferImport(preflight.planToken)
    } catch (error) {
      execution = null
      preflight = null
      errorMessage = messageFrom(error, language === 'zh' ? '导入失败，请重新预检。' : 'Import failed. Run preflight again.')
    } finally {
      busy = false
    }
  }

  function importAnother() {
    bundlePath = ''
    inspection = null
    mappings = {}
    conflictPolicy = 'skip'
    preflight = null
    execution = null
    targetTool = 'codex'
    sourceTool = 'codex'
    reviewAccepted = false
    conversionAcknowledged = false
    errorMessage = ''
  }

  function folderFromPath(path: string) {
    return path.trim().replace(/[\\/][^\\/]*$/, '')
  }

  async function ensureTargetRoot(tool: 'codex' | 'claude') {
    if (targetRoots[tool]) return targetRoots[tool]
    const root = tool === 'codex' ? await DefaultCodexRoot() : await DefaultClaudeRoot()
    targetRoots = { ...targetRoots, [tool]: root }
    return root
  }

  function folderName(path: string) {
    return path.split(/[\\/]/).filter(Boolean).pop() || path
  }

  function formatBytes(bytes: number) {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 KB'
    return bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }
</script>

{#if execution}
  <main class="import-result-v6" lang={language === 'zh' ? 'zh-CN' : 'en'}>
    <header class="import-result-toolbar">
      <button type="button" class="import-result-back" on:click={() => dispatch('back')} aria-label={text.backHome}><ArrowLeft size={23} /></button>
      <strong>{text.title}</strong>
      <LanguageSwitch bind:language className="import-language-switch" />
    </header>
    <section class="import-result-layout">
      <div class="import-result-main">
        <span class:pending={execution.pendingDiscovery > 0} class="import-result-mark"><CircleCheck size={37} strokeWidth={2.1} /></span>
        <h1>{execution.pendingDiscovery > 0 ? text.pendingTitle : text.resultTitle}</h1>
        <p>{execution.pendingDiscovery > 0 ? text.pendingSubtitle(execution.targetTool === 'claude' ? 'Claude Code' : 'Codex') : text.resultSubtitle}</p>
        <div class="import-result-stats" aria-label={text.resultTitle}>
          <div><strong>{execution.written + execution.replaced}</strong><span>{text.filesWritten}</span></div>
          <div><strong>{execution.filesVerified ? '✓' : '—'}</strong><span>{text.filesVerified}</span></div>
          <div><strong>{execution.discovered}</strong><span>{text.discoveredBy(execution.targetTool === 'claude' ? 'Claude Code' : 'Codex')}</span></div>
          <div class:pending={execution.pendingDiscovery > 0}><strong>{execution.pendingDiscovery}</strong><span>{text.pendingDiscovery}</span></div>
        </div>
        <div class="import-result-detail"><span>{text.create} {execution.written}</span><span>{text.skipped} {execution.skipped}</span><span>{text.replaced} {execution.replaced}</span></div>
        {#if execution.warnings?.length}
          <section class="import-result-warnings"><h2>{text.warnings}</h2>{#each execution.warnings as warning}<p>{warning}</p>{/each}</section>
        {/if}
        <div class="import-result-actions"><button type="button" class="import-v6-back" on:click={() => dispatch('back')}>{text.backHome}</button><button type="button" class="import-v6-continue" on:click={importAnother}><RefreshCw size={18} />{text.importAnother}</button></div>
      </div>
      <aside class="import-result-aside"><img src="/assets/import/import-mole-inspect.png" alt="" /><strong>{execution.message}</strong></aside>
    </section>
  </main>
{:else}
  <main class="import-v6" lang={language === 'zh' ? 'zh-CN' : 'en'}>
    <header class="import-v6-progress-shell">
      <div class="import-v6-progress-row">
        <nav class="import-v6-steps" aria-label={text.progress}>
          {#each text.steps as label, index}
            <div class:current={importStep === index + 1} class:done={importStep > index + 1} class="import-v6-step"><span>{index + 1}</span><strong>{label}</strong></div>
            {#if index < 4}<i aria-hidden="true"></i>{/if}
          {/each}
        </nav>
        <LanguageSwitch bind:language className="import-language-switch" />
      </div>
    </header>

    <section class="import-v6-scroll">
      {#if inspection && !reviewAccepted}
        <div class="import-review-shell">
          {#if errorMessage}<div class="inline-notice error import-review-notice" role="alert"><span class="status-icon">!</span>{errorMessage}</div>{/if}
          <ImportReview
            bind:targetTool
            bind:acknowledged={conversionAcknowledged}
            {language}
            {sourceTool}
            bundlePath={inspection.bundlePath}
            projectCount={inspection.projectCount}
            conversationCount={inspection.conversationCount}
            projectNames={inspection.sources.map((source) => source.name)}
          />
        </div>
      {:else}
        <div class="import-v6-grid">
        <section class="import-v6-left" aria-labelledby="choose-package-title">
          <div class="import-v6-heading"><h1>{text.title}</h1><p>{text.description}</p><img class="import-v6-mascot" src="/assets/import/import-mole-restore.png" alt="" /></div>
          {#if errorMessage}<div class="inline-notice error import-v6-notice" role="alert"><span class="status-icon">!</span>{errorMessage}</div>{/if}

          {#if !inspection}
            <div class="import-v6-dropzone"><FileArchive size={108} strokeWidth={1.35} aria-hidden="true" /><h2 id="choose-package-title">{text.choosePackage}</h2><button type="button" class="import-v6-primary" disabled={busy} on:click={chooseImportFile}>{text.chooseFile}</button></div>
            <div class="import-v6-divider"><span>{text.manualPath}</span></div>
            <div class="import-v6-path-row"><input id="bundle-path" bind:value={bundlePath} on:input={clearInspection} placeholder={text.placeholder} aria-label={text.placeholder} /><button type="button" class="import-v6-outline" disabled={!bundlePath.trim() || busy} on:click={inspectBundle}>{busy ? text.checking : text.checkFile}</button></div>
          {:else}
            <section class="import-v6-status" role="status"><span class="import-v6-status-mark">✓</span><div><strong>{text.packageReady}</strong><p>{inspection.projectCount} {text.projects} · {inspection.conversationCount} {text.conversations}</p><small>{text.sourceWarning}</small></div></section>
            <section class="import-v6-followup import-v6-mapping">
              <h2>{text.mappingTitle}</h2><p>{text.mappingDescription}</p>
              {#each inspection.sources as source}
                <div class="import-v6-mapping-item">
                  <div class="import-v6-mapping-source"><strong>{source.name}</strong><small>{source.sourceFolder} · {source.conversationCount} {text.conversations}</small></div>
                  <div class="import-v6-mapping-controls">
                    <select aria-label={`${source.name} ${text.chooseTarget}`} value={mappings[source.key]?.targetId || (mappings[source.key]?.targetDirectory ? '__custom__' : '')} on:change={(event) => selectTarget(source.key, event)}>
                      <option value="">{text.chooseTarget}</option>
                      {#each inspection.targets as target}<option value={target.id}>{target.name} · {target.folder}</option>{/each}
                      {#if mappings[source.key]?.targetDirectory}<option value="__custom__">{text.selectedFolder}: {folderName(mappings[source.key].targetDirectory)}</option>{/if}
                    </select>
                    <button type="button" class="import-v6-folder-button" disabled={busy} on:click={() => chooseTargetFolder(source.key)}><FolderOpen size={16} />{text.chooseFolder}</button>
                  </div>
                </div>
              {/each}
              {#if !allMappingsReady}<p class="import-v6-mapping-warning">{text.mappingMissing}</p>{/if}
              <fieldset class="import-v6-conflicts"><legend>{text.conflictTitle}</legend><label><input type="radio" bind:group={conflictPolicy} value="skip" on:change={() => preflight = null} /><span><strong>{text.skip}</strong></span></label><label><input type="radio" bind:group={conflictPolicy} value="replace" on:change={() => preflight = null} /><span><strong>{text.replace}</strong><small>{text.replaceWarning}</small></span></label></fieldset>
              <button type="button" class="import-v6-primary" disabled={!allMappingsReady || busy} on:click={prepareImport}>{busy ? text.preflighting : text.runPreflight}</button>
            </section>
          {/if}

          {#if preflight}
            <section class:validation-error={!preflight.canExecute} class="import-v6-status import-v6-preflight" role="status"><span class="import-v6-status-mark">{preflight.canExecute ? '✓' : '!'}</span><div><strong>{preflight.toolRunning ? text.toolRunning(targetToolName) : preflight.message}</strong><p>{preflight.projectCount} {text.projects} · {preflight.conversationCount} {text.conversations} · {text.required} {formatBytes(preflight.requiredBytes)} · {text.available} {formatBytes(preflight.availableBytes)}</p><small>{text.create} {preflight.createCount} · {text.skipped} {preflight.skipCount} · {text.replaced} {preflight.replaceCount}</small>{#if preflight.canExecute}<button type="button" class="import-v6-primary" disabled={busy} on:click={executeImport}>{busy ? text.importing : text.startImport}</button>{:else}<button type="button" class="import-v6-outline" disabled={busy} on:click={prepareImport}>{text.retryPreflight}</button>{/if}</div></section>
          {/if}
        </section>

        <aside class="import-v6-summary">
          <h2>{text.scopeTitle}</h2>
          <section class="import-v6-scope-group"><h3>{text.included}</h3><p><span class="included"><Check size={17} strokeWidth={3} /></span>{text.history}</p><p><span class="included"><Check size={17} strokeWidth={3} /></span>{text.messages}</p></section>
          <section class="import-v6-scope-group excluded"><h3>{text.notIncluded}</h3><p><span class="not-included"><Minus size={16} strokeWidth={3} /></span>{text.excluded}</p></section>
          <section class="import-v6-before"><h3>{text.beforeImport}</h3><p class="import-v6-note"><ShieldCheck size={21} strokeWidth={2.2} aria-hidden="true" /><span>{text.packageChecked}</span></p></section>
        </aside>
      </div>
      {/if}
    </section>
    {#if inspection && !reviewAccepted}
      <footer class="import-v6-footer"><button type="button" class="import-v6-back" on:click={returnToPackage}>{text.back}</button><button type="button" class="import-v6-continue" disabled={busy || (isCrossTool && !conversionAcknowledged)} on:click={acceptReview}>{isCrossTool ? text.acceptConversion : text.continue}</button></footer>
    {:else}
      <footer class="import-v6-footer"><button type="button" class="import-v6-back" on:click={() => dispatch('back')}>{text.backHome}</button><button type="button" class="import-v6-continue" disabled={!preflight?.canExecute || busy} on:click={executeImport}>{busy ? text.importing : text.continue}</button></footer>
    {/if}
  </main>
{/if}
