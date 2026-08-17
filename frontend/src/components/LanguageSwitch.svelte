<script lang="ts">
  export let language: 'en' | 'zh'
  export let className = ''
</script>

<div class={`global-language-switch ${className}`.trim()} role="group" aria-label={language === 'zh' ? '语言' : 'Language'}>
  <button
    class:active={language === 'en'}
    type="button"
    aria-pressed={language === 'en'}
    on:click={() => { language = 'en' }}
  >EN</button>
  <button
    class:active={language === 'zh'}
    type="button"
    aria-pressed={language === 'zh'}
    on:click={() => { language = 'zh' }}
  >中文</button>
</div>
