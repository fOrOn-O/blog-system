import DOMPurify from 'dompurify'

// 与普通文章展示共用的 HTML 渲染边界；不会代替 Go 写入校验。
export const sanitizeArticleHTML = html => DOMPurify.sanitize(html || '', { USE_PROFILES: { html: true } })
