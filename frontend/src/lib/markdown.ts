import DOMPurify from 'dompurify'
import { marked } from 'marked'

const ALLOWED_MARKDOWN_TAGS = [
  'p',
  'br',
  'strong',
  'em',
  'del',
  'blockquote',
  'ul',
  'ol',
  'li',
  'pre',
  'code',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'hr',
  'table',
  'thead',
  'tbody',
  'tr',
  'th',
  'td',
]

export function renderPreviewMarkdown(source: string): string {
  if (!source.trim()) return ''

  const html = marked.parse(source, {
    async: false,
    breaks: true,
    gfm: true,
  })

  return DOMPurify.sanitize(html, {
    ALLOWED_ATTR: [],
    ALLOWED_TAGS: ALLOWED_MARKDOWN_TAGS,
    KEEP_CONTENT: true,
  })
}
