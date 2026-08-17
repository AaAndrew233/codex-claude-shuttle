<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { TransferTool } from '../lib/transfer-tools'

  export let value: TransferTool = 'codex'
  export let ariaLabel = 'Transfer tool'

  const dispatch = createEventDispatcher<{ change: TransferTool }>()

  function selectTool(tool: TransferTool) {
    if (tool === value) return
    value = tool
    dispatch('change', tool)
  }
</script>

<div class="transfer-tool-switch" role="group" aria-label={ariaLabel}>
  <button type="button" class:active={value === 'codex'} aria-pressed={value === 'codex'} on:click={() => selectTool('codex')}>Codex</button>
  <button type="button" class:active={value === 'claude'} aria-pressed={value === 'claude'} on:click={() => selectTool('claude')}>Claude Code</button>
</div>
