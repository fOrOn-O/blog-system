import { Marked } from 'marked'
import DOMPurify from 'dompurify'

const parser = new Marked({ gfm: true, breaks: false, async: false })

export function renderAssistantMarkdown(text, sanitize = html => DOMPurify.sanitize(html, {
  // 只保留消息排版；不允许图片请求、表单、style、SVG 或事件属性。
  ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'del', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code',
    'a', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'hr'],
  ALLOWED_ATTR: ['href', 'title', 'start'],
  ALLOW_DATA_ATTR: false,
  ALLOW_ARIA_ATTR: false,
})) {
  // sanitize 是最终步骤，清洗后不再拼接或改写 HTML。
  return sanitize(parser.parse(typeof text === 'string' ? text : ''))
}
