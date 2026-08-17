import { describe, expect, it } from 'vitest'

import { renderPreviewMarkdown } from './markdown'

describe('renderPreviewMarkdown', () => {
  it('renders the Markdown structures used by Codex replies', () => {
    const html = renderPreviewMarkdown(`## Result

Use **bold text** and \`inline code\`.

- First item
- Second item

\`\`\`text
safe output
\`\`\``)

    expect(html).toContain('<h2>Result</h2>')
    expect(html).toContain('<strong>bold text</strong>')
    expect(html).toContain('<code>inline code</code>')
    expect(html).toContain('<ul>')
    expect(html).toContain('<pre><code>safe output')
  })

  it('removes executable markup, external resources, links, and attributes', () => {
    const html = renderPreviewMarkdown(`Before

<script>alert('unsafe')</script>

[external](javascript:alert('unsafe'))

<img src="https://example.com/tracker.png" onerror="alert('unsafe')">

<p class="spoofed" style="position:fixed">After</p>`)

    expect(html).toContain('Before')
    expect(html).toContain('external')
    expect(html).toContain('After')
    expect(html).not.toMatch(/<script|<img|<a|javascript:|onerror=|class=|style=/i)
  })
})
