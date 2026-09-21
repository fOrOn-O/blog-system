import { test, expect } from '@playwright/test'
import { mkdir } from 'node:fs/promises'

const go = 'http://127.0.0.1:18080/api/v1'
async function editor(page, request) {
  const response = await request.post(`${go}/auth/login`, { data: { username: 'user0', password: 'e2e-test-password' } })
  const { token, user } = (await response.json()).data
  const headers = { Authorization: `Bearer ${token}` }
  const created = await request.post(`${go}/articles/drafts`, { headers, data: {
    title: '结构优化回归', content: '<p>开篇说明</p><h2>细节</h2><p>需要保留的正文。</p><p>原始末段</p>'
  } })
  expect(created.status()).toBe(201)
  const article = (await created.json()).data
  await page.addInitScript(({ token, user }) => {
    localStorage.setItem('blog_token', token)
    localStorage.setItem('blog_user', JSON.stringify(user))
  }, { token, user })
  await page.goto(`/article/edit/${article.id}`)
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  return { article, headers }
}

test('整体正文结构优化生成完整提案，预览和明确人工批准前不写入', async ({ page, request }) => {
  const { article, headers } = await editor(page, request)
  await page.getByLabel('助手模式').selectOption('write')
  await page.getByLabel('向助手提问').fill('帮我整体优化一下正文结构')
  const response = page.waitForResponse(r => r.url().endsWith('/chat'))
  await page.getByRole('button', { name: '发送', exact: true }).click()
  const result = await response
  expect(result.status()).toBe(200)
  const data = await result.json()
  expect(data.proposal.proposed_content).toContain(article.content)
  expect(data.answer).not.toMatch(/list_my_articles|get_version_diff|search_article_version|submit_article_edit_proposal/)
  const apply = page.getByRole('button', { name: '应用修改', exact: true })
  await expect(apply).toBeDisabled()
  const read = async () => (await (await request.get(`${go}/user/articles/${article.id}`, { headers })).json()).data
  expect((await read()).version).toBe(1)
  await page.getByRole('button', { name: '预览修改', exact: true }).click()
  await expect(apply).toBeEnabled()
  expect((await read()).content).toBe(article.content)
  await apply.click()
  await expect(page.getByRole('dialog')).toContainText('不会自动发布')
  expect((await read()).version).toBe(1)
  await page.getByRole('dialog').getByRole('button', { name: '确认应用', exact: true }).click()
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  expect((await read()).published_version).toBe(0)
})

const markdown = '段落 **粗体** *斜体* `inline` [文档](https://example.com)\n\n- 一\n- 二\n\n1. 有序\n\n> 引用\n\n```js\nconst value = "' + 'long'.repeat(150) + '"\n```\n\n| 项目 | 说明 |\n| --- | --- |\n| 缓存 | RDB |\n\n'
const malicious = '<img src=x onerror="window.xss=1"><script>window.xss=1</script><svg onload="window.xss=1"></svg><iframe srcdoc="evil"></iframe><a href="javascript:window.xss=1">危险链接</a><p style="position:fixed" onclick="window.xss=1">文本</p>'

test('助手 Markdown 正确排版且经过真实 DOMPurify 清洗，两处回答共用安全组件', async ({ page, request }) => {
  await editor(page, request)
  await page.route('**/chat', route => route.fulfill({ json: { answer: markdown + malicious, proposal: null, has_evidence: true, sources: [] } }))
  await page.getByLabel('向助手提问').fill('查看当前版本')
  await page.getByRole('button', { name: '发送', exact: true }).click()
  const verify = async container => {
    for (const tag of ['strong', 'em', 'ul', 'ol', 'blockquote', 'pre code', 'table']) await expect(container.locator(tag)).toHaveCount(1)
    await expect(container.getByRole('link', { name: '文档' })).toHaveAttribute('href', 'https://example.com')
    await expect(container.locator('img, script, svg, iframe, [onclick], [style], [href^="javascript:"]')).toHaveCount(0)
    expect(await page.evaluate(() => window.xss)).toBeUndefined()
    const pre = container.locator('pre')
    expect(await pre.evaluate(el => el.scrollWidth > el.clientWidth)).toBeTruthy()
    expect(await container.evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBeTruthy()
  }
  await verify(page.locator('.agent-panel .assistant-markdown'))
  await page.goto('/knowledge')
  await page.getByLabel('你的问题').fill('查看站内文章')
  await page.getByRole('button', { name: '提问', exact: true }).click()
  await verify(page.locator('.knowledge-page .assistant-markdown'))
})

test('面板展开收起、输入高度上限、长消息滚动和移动端关闭', async ({ page, request }) => {
  await editor(page, request)
  const panel = page.getByRole('complementary', { name: 'Agent 助手' })
  await page.getByRole('button', { name: '展开助手' }).click()
  await expect(page.getByRole('button', { name: '收起助手' })).toHaveAttribute('aria-expanded', 'true')
  expect((await panel.boundingBox()).height).toBeGreaterThan(700)
  const input = page.getByLabel('向助手提问')
  await input.fill('一行输入\n'.repeat(150))
  await expect.poll(() => input.evaluate(el => el.clientHeight)).toBeLessThanOrEqual(140)
  expect(await input.evaluate(el => el.scrollHeight > el.clientHeight)).toBeTruthy()
  const messages = page.getByLabel('助手消息', { exact: true })
  expect((await messages.boundingBox()).height).toBeGreaterThan(200)
  let release
  let count = 0
  const pending = new Promise(resolve => { release = resolve })
  await page.route('**/chat', async route => {
    if (++count === 2) await pending
    await route.fulfill({ json: { answer: '消息段落\n\n'.repeat(90), proposal: null, sources: [] } })
  })
  await input.fill('第一次')
  await page.getByRole('button', { name: '发送', exact: true }).click()
  await expect(panel.locator('.assistant-markdown')).toHaveCount(1)
  await expect.poll(() => messages.evaluate(el => el.scrollHeight - el.scrollTop - el.clientHeight)).toBeLessThan(40)
  await input.fill('第二次')
  await page.getByRole('button', { name: '发送', exact: true }).click()
  await messages.evaluate(el => { el.scrollTop = 0; el.dispatchEvent(new Event('scroll')) })
  release()
  await expect(panel.locator('.assistant-markdown')).toHaveCount(2)
  await expect.poll(() => messages.evaluate(el => el.scrollTop)).toBeLessThan(40)
  await mkdir('../output/playwright', { recursive: true })
  await page.screenshot({ path: '../output/playwright/production-fix-panel.png' })
  await page.getByRole('button', { name: '收起助手' }).click()
  await expect(page.getByRole('button', { name: '展开助手' })).toHaveAttribute('aria-expanded', 'false')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: '展开助手' }).click()
  const box = await panel.boundingBox()
  expect(box.x).toBeGreaterThanOrEqual(0)
  expect(box.x + box.width).toBeLessThanOrEqual(390)
  await page.getByRole('button', { name: '收起助手' }).click()
  await expect(page.getByRole('button', { name: '展开助手' })).toBeVisible()
})

test('搜索仅一个清除按钮，Enter 与提交按钮一致，空查询不提交，导航及 favicon 正确', async ({ page, request }) => {
  await editor(page, request)
  const search = page.getByRole('search')
  const input = search.getByLabel('搜索文章')
  await expect(input).toHaveAttribute('type', 'text')
  await expect(search.getByRole('button', { name: '清空搜索' })).toHaveCount(0)
  const submit = search.getByRole('button', { name: '搜索', exact: true })
  await expect(submit).toBeDisabled()
  await input.fill('   ')
  const original = page.url()
  await input.press('Enter')
  expect(page.url()).toBe(original)
  await input.fill('redis')
  await expect(search.getByRole('button', { name: '清空搜索' })).toHaveCount(1)
  await search.getByRole('button', { name: '清空搜索' }).click()
  await expect(input).toHaveValue('')
  await input.fill(' redis ')
  await input.press('Enter')
  await expect(page).toHaveURL(/\/search\?keyword=redis$/)
  await input.fill('缓存')
  await submit.click()
  await expect.poll(() => new URL(page.url()).searchParams.get('keyword')).toBe('缓存')
  await page.getByRole('navigation', { name: '主导航' }).getByRole('button', { name: '知识小助手', exact: true }).click()
  await expect(page).toHaveURL('/knowledge')
  const icon = page.locator('link[rel=icon]')
  await expect(icon).toHaveAttribute('href', '/favicon-face-v1.png')
  const asset = await request.get('/favicon-face-v1.png')
  expect(asset.status()).toBe(200)
  expect(asset.headers()['content-type']).toContain('image/png')
})

test('助手状态码错误提示保持脱敏且可重试', async ({ page, request }) => {
  await editor(page, request)
  for (const [status, expected] of [[403, '你没有权限执行此操作。'], [429, '请求过于频繁，请在 17 秒后重试。'],
    [422, '请求无法处理，请检查输入和所选文章版本。'], [502, '助手服务暂时失败，请稍后重试。'], [504, '助手响应超时，请稍后重试。']]) {
    await page.route('**/chat', route => route.fulfill({ status, headers: { 'Retry-After': '17' }, json: { detail: 'SECRET provider body' } }))
    await page.getByLabel('向助手提问').fill('请求')
    await page.getByRole('button', { name: '发送', exact: true }).click()
    await expect(page.locator('.agent-panel [role=alert]')).toHaveText(expected)
    await expect(page.getByRole('button', { name: '发送', exact: true })).not.toHaveAttribute('aria-busy', 'true')
    expect(await page.locator('body').innerText()).not.toContain('SECRET')
    await page.unroute('**/chat')
  }
})

test('我的文章 draft 标题进入 owner 工作版本，published 标题仍进入公开详情', async ({ page, request }) => {
  const { article, headers } = await editor(page, request)
  await request.put(`${go}/articles/${article.id}/draft`, { headers, data: { expected_version: 1, title: `私有草稿导航 ${article.id}` } })
  const published = (await (await request.post(`${go}/articles`, { headers, data: { title: `公开导航 ${article.id}`, content: '<p>公开正文</p>' } })).json()).data
  await page.goto('/my-articles')
  await page.getByRole('heading', { name: `私有草稿导航 ${article.id}`, exact: true }).click()
  await expect(page).toHaveURL(`/article/edit/${article.id}`)
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByLabel('文章正文编辑器')).toContainText('需要保留的正文')
  await expect(page.locator('.el-message--error')).toHaveCount(0)
  await page.goto('/my-articles')
  await page.getByRole('heading', { name: `公开导航 ${article.id}`, exact: true }).click()
  await expect(page).toHaveURL(`/article/${published.id}`)
  await expect(page.locator('.article-content')).toContainText('公开正文')
})

test('草稿仍受 Go 权限保护，匿名公开读取 404，非 owner 工作区读取 403', async ({ page, request, browser }) => {
  const { article } = await editor(page, request)
  const publicRead = await request.get(`${go}/articles/${article.id}`)
  expect(publicRead.status()).toBe(404)
  expect(await publicRead.text()).not.toContain(article.content)
  const registered = await request.post(`${go}/auth/register`, { data: {
    username: 'draft-navigation-other', email: 'draft-navigation-other@example.test', password: 'e2e-test-password'
  } })
  expect(registered.status()).toBe(201)
  const { token, user } = (await registered.json()).data
  const forbidden = await request.get(`${go}/user/articles/${article.id}`, { headers: { Authorization: `Bearer ${token}` } })
  expect(forbidden.status()).toBe(403)
  expect(await forbidden.text()).not.toContain(article.content)
  const anonymous = await browser.newContext()
  const other = await browser.newContext()
  try {
    const publicPage = await anonymous.newPage()
    await publicPage.goto(`http://127.0.0.1:15173/article/${article.id}`)
    await expect(publicPage.locator('.detail-page [role=alert]')).toHaveText('文章不存在或当前不可访问。')
    await expect(publicPage).toHaveURL(`/article/${article.id}`)
    await expect(publicPage.locator('.el-message--error')).toHaveCount(0)
    await other.addInitScript(({ token, user }) => {
      localStorage.setItem('blog_token', token)
      localStorage.setItem('blog_user', JSON.stringify(user))
    }, { token, user })
    const ownerPage = await other.newPage()
    await ownerPage.goto(`http://127.0.0.1:15173/article/edit/${article.id}`)
    await expect(ownerPage.getByRole('alert')).toContainText('你没有权限查看这篇文章。')
    await expect(ownerPage).toHaveURL(`/article/edit/${article.id}`)
    await expect(ownerPage.locator('.el-message--error')).toHaveCount(0)
    expect(await ownerPage.locator('body').innerText()).not.toContain('需要保留的正文')
  } finally { await anonymous.close(); await other.close() }
})

test('真正不存在的文章保留路由、只显示一次错误，owner 登录过期仍走统一登录处理', async ({ page, request }) => {
  await editor(page, request)
  for (const path of ['/article/999999', '/article/edit/999999']) {
    await page.goto(path)
    await expect(page.getByRole('alert')).toContainText('文章不存在或当前不可访问。')
    await expect(page.getByRole('alert')).toHaveCount(1)
    await expect(page.locator('.el-message--error')).toHaveCount(0)
    await expect(page).toHaveURL(path)
    await expect(page.getByRole('button', { name: '重新加载', exact: true })).toBeVisible()
  }
  await page.route('**/user/articles/*', route => route.fulfill({ status: 401, json: { code: 401, message: 'expired' } }))
  await page.goto('/article/edit/999999')
  await expect(page).toHaveURL(/\/login\?redirect=/)
  expect(await page.evaluate(() => localStorage.getItem('blog_token'))).toBeNull()
})
