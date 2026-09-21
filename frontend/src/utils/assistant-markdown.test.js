import test from 'node:test'
import assert from 'node:assert/strict'
import { renderAssistantMarkdown } from './assistant-markdown.js'

test('Markdown 解析支持段落、列表、强调、链接、引用、代码和表格，并将完整结果交给最终清洗步骤', () => {
  let parsed
  const text = '段落 **粗体** *斜体* `inline` [链接](https://example.com)\n\n- 一\n- 二\n\n1. 有序\n\n> 引用\n\n```js\nconst a = 1\n```\n\n| A | B |\n| --- | --- |\n| C | D |'
  const result = renderAssistantMarkdown(text, html => { parsed = html; return 'sanitized-result' })
  for (const tag of ['p', 'strong', 'em', 'code', 'a', 'ul', 'li', 'ol', 'blockquote', 'pre', 'table', 'th', 'td']) {
    assert.match(parsed, new RegExp(`<${tag}(>| )`))
  }
  assert.equal(result, 'sanitized-result')
})

test('代码围栏中的 HTML 保持文字，不作为元素生成', () => {
  const html = renderAssistantMarkdown('```html\n<img src=x onerror=alert(1)>\n```', text => text)
  assert.ok(html.includes('&lt;img'))
  assert.ok(!html.includes('<img'))
})
