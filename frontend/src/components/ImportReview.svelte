<script lang="ts">
  import { ArrowRight, Check, CircleAlert, CircleCheck, FileArchive, Info, Minus } from '@lucide/svelte'
  import type { TransferTool } from '../lib/transfer-tools'
  import TransferToolSwitch from './TransferToolSwitch.svelte'

  export let language: 'en' | 'zh' = 'en'
  export let sourceTool: TransferTool = 'codex'
  export let targetTool: TransferTool = 'codex'
  export let bundlePath = ''
  export let projectCount = 0
  export let conversationCount = 0
  export let projectNames: string[] = []
  export let acknowledged = false

  const copy = {
    en: {
      reviewImport: 'Review import',
      reviewConversion: 'Review conversion',
      description: 'Review the transfer package and import settings before continuing.',
      direction: 'Migration direction',
      source: 'Source (auto-detected)',
      destination: 'Destination',
      nativeStatus: 'Complete native conversation',
      conversionStatus: 'Package validated successfully',
      packageTitle: 'Validated transfer package',
      validationComplete: 'Package validated successfully',
      projects: 'projects',
      conversations: 'conversations',
      projectSummary: 'Project summary',
      scope: 'Import scope',
      conversionScope: 'Conversion scope',
      included: 'Included',
      notIncluded: 'Not included',
      notConverted: 'Not converted',
      history: 'Conversation history',
      nativeReplies: (tool: string) => `User messages and ${tool} replies`,
      nativeEvents: 'Native tool events',
      projectBoundary: 'Project source and account settings',
      userMessages: 'User messages',
      finalReplies: 'Final assistant replies',
      titles: 'Conversation titles and timestamps',
      toolCalls: 'Tool calls and results',
      reasoning: 'Internal reasoning',
      subagents: 'Subagent sessions',
      settings: 'Memory, skills, settings and attachments',
      conversionSummary: (source: string, target: string) => `Source: ${source} -> Target: ${target}`,
      conversionValidation: 'Validation complete',
      conversionNotice: 'You are reviewing the transfer package details and conversion scope. Confirm that you understand what will not be converted before continuing.',
      beforeImport: 'Before import',
      closeTool: (tool: string) => `${tool} must be closed before files are written.`,
      acknowledge: 'I understand these items will not be converted.',
    },
    zh: {
      reviewImport: '检查导入内容',
      reviewConversion: '检查转换内容',
      description: '继续前请核对迁移包和导入设置。',
      direction: '迁移方向',
      source: '来源（自动识别）',
      destination: '目标工具',
      nativeStatus: '保留完整原生对话',
      conversionStatus: '迁移包检查通过',
      packageTitle: '已验证的迁移包',
      validationComplete: '迁移包检查通过',
      projects: '个项目',
      conversations: '个对话',
      projectSummary: '项目摘要',
      scope: '导入范围',
      conversionScope: '转换范围',
      included: '包含内容',
      notIncluded: '不包含',
      notConverted: '不会转换',
      history: '完整对话历史',
      nativeReplies: (tool: string) => `用户消息和 ${tool} 回复`,
      nativeEvents: '原生工具事件',
      projectBoundary: '项目源码和账号设置',
      userMessages: '用户消息',
      finalReplies: '助手最终回复',
      titles: '对话标题和时间',
      toolCalls: '工具调用和结果',
      reasoning: '内部推理',
      subagents: '子代理会话',
      settings: 'Memory、Skills、设置和附件',
      conversionSummary: (source: string, target: string) => `来源：${source} -> 目标：${target}`,
      conversionValidation: '检查完成',
      conversionNotice: '你正在核对迁移包详情和转换范围。继续前，请确认已了解哪些内容不会被转换。',
      beforeImport: '导入前',
      closeTool: (tool: string) => `写入文件前必须完全退出 ${tool}。`,
      acknowledge: '我已了解这些内容不会被转换。',
    },
  } as const

  $: text = copy[language]
  $: sameTool = sourceTool === targetTool
  $: sourceName = toolName(sourceTool)
  $: targetName = toolName(targetTool)
  $: fileName = bundlePath.split(/[\\/]/).filter(Boolean).pop() || `${sourceName}-conversations.zip`
  $: visibleProjectNames = projectNames.filter(Boolean).slice(0, 3)

  function toolName(tool: TransferTool) {
    return tool === 'codex' ? 'Codex' : 'Claude Code'
  }

  function changeTarget(event: CustomEvent<TransferTool>) {
    targetTool = event.detail
    acknowledged = false
  }
</script>

<div class:conversion={!sameTool} class="import-review-grid">
  <section class="import-review-main" aria-labelledby="import-review-title">
    <div class="import-review-heading">
      <h1 id="import-review-title">{sameTool ? text.reviewImport : text.reviewConversion}</h1>
      <p>{text.description}</p>
    </div>

    <section class="import-review-direction">
      <h2>{text.direction}</h2>
      <div class="import-review-direction-row">
        <div class="import-review-source"><span>{text.source}</span><strong>{sourceName}</strong></div>
        <ArrowRight class="import-review-arrow" size={34} strokeWidth={1.8} aria-hidden="true" />
        <div class="import-review-target"><span>{text.destination}</span><TransferToolSwitch bind:value={targetTool} ariaLabel={text.destination} on:change={changeTarget} /></div>
      </div>
      <p class="import-review-validation"><CircleCheck size={23} strokeWidth={2.1} />{sameTool ? text.nativeStatus : text.conversionStatus}</p>
    </section>

    <section class="import-review-package">
      {#if sameTool}<h2>{text.packageTitle}</h2>{/if}
      <div class:conversion={!sameTool} class="import-review-package-card">
        {#if sameTool}<FileArchive size={48} strokeWidth={1.5} aria-hidden="true" />{/if}
        <div>
          <strong>{fileName}</strong>
          <p>
            <span>{projectCount} {text.projects}</span>
            <span>{conversationCount} {text.conversations}</span>
            {#if sameTool && visibleProjectNames.length}<span>{text.projectSummary}: {visibleProjectNames.join(', ')}</span>{/if}
            {#if !sameTool}<span>{text.conversionSummary(sourceName, targetName)}</span>{/if}
          </p>
          {#if !sameTool}<small>{text.conversionValidation}</small>{/if}
        </div>
        {#if sameTool}<CircleCheck class="import-review-file-check" size={22} strokeWidth={2.3} aria-hidden="true" />{/if}
      </div>
      {#if sameTool}<p class="import-review-complete">{text.validationComplete}</p>{/if}
      {#if !sameTool}<p class="import-review-conversion-notice"><Info size={24} strokeWidth={2.1} />{text.conversionNotice}</p>{/if}
    </section>
  </section>

  <aside class="import-review-scope">
    <h2>{sameTool ? text.scope : text.conversionScope}</h2>
    <section class="import-review-scope-group included">
      <h3>{text.included}</h3>
      {#if sameTool}
        <p><span><Check size={16} strokeWidth={3} /></span>{text.history}</p>
        <p><span><Check size={16} strokeWidth={3} /></span>{text.nativeReplies(sourceName)}</p>
        <p><span><Check size={16} strokeWidth={3} /></span>{text.nativeEvents}</p>
      {:else}
        <p><span><Check size={16} strokeWidth={3} /></span>{text.userMessages}</p>
        <p><span><Check size={16} strokeWidth={3} /></span>{text.finalReplies}</p>
        <p><span><Check size={16} strokeWidth={3} /></span>{text.titles}</p>
      {/if}
    </section>

    <section class:warning={!sameTool} class="import-review-scope-group excluded">
      <h3>{sameTool ? text.notIncluded : text.notConverted}</h3>
      {#if sameTool}
        <p><span><Minus size={15} strokeWidth={3} /></span>{text.projectBoundary}</p>
      {:else}
        <p><span><CircleAlert size={15} strokeWidth={2.8} /></span>{text.toolCalls}</p>
        <p><span><CircleAlert size={15} strokeWidth={2.8} /></span>{text.reasoning}</p>
        <p><span><CircleAlert size={15} strokeWidth={2.8} /></span>{text.subagents}</p>
        <p><span><CircleAlert size={15} strokeWidth={2.8} /></span>{text.settings}</p>
      {/if}
    </section>

    <section class="import-review-before">
      <h3>{text.beforeImport}</h3>
      <p><Info size={22} strokeWidth={2.1} />{text.closeTool(targetName)}</p>
      {#if !sameTool}
        <label><input type="checkbox" bind:checked={acknowledged} />{text.acknowledge}</label>
      {/if}
    </section>

    <img class="import-review-mascot" src={sameTool ? '/assets/import/import-mole-native-review.png' : '/assets/import/import-mole-conversion-review.png'} alt="" />
  </aside>
</div>
