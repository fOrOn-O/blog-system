import { test, expect } from '@playwright/test'

const go = 'http://127.0.0.1:18080/api/v1'
const agent = 'http://127.0.0.1:18000'

async function login(page, request) {
  const response = await request.post(`${go}/auth/login`, { data: { username: 'user0', password: 'e2e-test-password' } })
  const { token, user } = (await response.json()).data
  await page.addInitScript(({ token, user }) => {
    localStorage.setItem('blog_token', token)
    localStorage.setItem('blog_user', JSON.stringify(user))
  }, { token, user })
  return { Authorization: `Bearer ${token}` }
}

async function ask(page, query = '缓存持久化') {
  await page.getByLabel('你的问题').fill(query)
  const response = page.waitForResponse(r => r.url().endsWith('/knowledge/chat'))
  await page.getByRole('button', { name: '提问', exact: true }).click()
  expect((await response).status()).toBe(200)
  await expect(page.getByRole('region', { name: '知识回答' })).toBeVisible()
}

test('真实 Vue/Python/Go：跨文章来源、未发布工作稿隔离、发布后独立同步、归档过滤及 exact 问答', async ({ page, request }) => {
  const headers = await login(page, request)
  const articles = []
  for (const [title, content] of [['知识 E2E A', '<p>RDB 保存快照。</p>'], ['知识 E2E B', '<p>AOF 保存写入日志。</p>']]) {
    const response = await request.post(`${go}/articles`, { headers, data: { title, content } })
    expect(response.status()).toBe(201)
    const article = (await response.json()).data
    articles.push(article)
    expect((await request.post(`${agent}/api/v1/agent/knowledge/sync/${article.id}`, { headers })).status()).toBe(200)
  }
  const [a, b] = articles
  await page.goto('/knowledge')
  await ask(page)
  const sources = page.getByRole('region', { name: '回答来源' })
  await expect(sources.getByRole('link', { name: a.title, exact: true })).toHaveAttribute('href', `/article/${a.id}`)
  await expect(sources.getByRole('link', { name: b.title, exact: true })).toBeVisible()
  await expect(page.getByRole('region', { name: '知识回答' })).toContainText('RDB 保存快照')

  expect((await request.put(`${go}/articles/${a.id}/draft`, { headers, data: { expected_version: 1, content: '<p>未发布的新快照策略。</p>' } })).status()).toBe(200)
  await ask(page)
  await expect(page.getByRole('region', { name: '知识回答' })).not.toContainText('未发布的新快照策略')
  await expect(sources).toContainText(`#${a.id} / V1`)

  await page.goto(`/article/edit/${a.id}`)
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  const synced = page.waitForResponse(r => r.url().endsWith(`/knowledge/sync/${a.id}`))
  await page.getByRole('button', { name: '发布工作版本', exact: true }).click()
  await page.getByRole('dialog', { name: '确认发布' }).getByRole('button', { name: '确认', exact: true }).click()
  expect((await synced).status()).toBe(200)
  await page.goto('/knowledge')
  await ask(page)
  await expect(sources).toContainText(`#${a.id} / V2`)
  await expect(sources).not.toContainText(`#${a.id} / V1`)
  await expect(page.getByRole('region', { name: '知识回答' })).toContainText('未发布的新快照策略')

  // exact 索引仍是独立显式动作，公开知识同步不自动创建 exact 索引。
  expect((await request.post(`${agent}/test/index/${a.id}/2`, { headers })).status()).toBe(200)
  await page.goto(`/article/edit/${a.id}`)
  await page.getByLabel('向助手提问').fill('当前版本的策略是什么？')
  await page.getByRole('button', { name: '发送', exact: true }).click()
  await expect(page.getByRole('region', { name: '回答来源' })).toContainText(`#${a.id} / V2`)
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)

  const archivedSync = page.waitForResponse(r => r.url().endsWith(`/knowledge/sync/${a.id}`))
  await page.getByRole('button', { name: '归档文章', exact: true }).click()
  await page.getByRole('dialog', { name: '确认归档' }).getByRole('button', { name: '确认', exact: true }).click()
  expect((await archivedSync).status()).toBe(200)
  await page.goto('/knowledge')
  await ask(page)
  await expect(sources.getByRole('link', { name: a.title, exact: true })).toHaveCount(0)
  await sources.getByRole('link', { name: b.title, exact: true }).click()
  await expect(page).toHaveURL(`/article/${b.id}`)
})

test('/knowledge 登录保护、无依据、失败和来源文本安全', async ({ page, request }) => {
  await page.goto('/knowledge')
  await expect(page).toHaveURL(/\/login\?redirect=\/knowledge/)
  await login(page, request)
  await page.goto('/knowledge')
  await page.route('**/knowledge/chat', route => route.fulfill({ json: { answer: '本站当前已发布文章中没有足够检索依据。', has_evidence: false, sources: [] } }))
  await ask(page, '没有依据的问题')
  await expect(page.getByRole('heading', { name: '暂无足够依据' })).toBeVisible()
  await expect(page.getByRole('region', { name: '回答来源' })).toHaveCount(0)
  await page.unroute('**/knowledge/chat')
  await page.route('**/knowledge/chat', route => route.fulfill({ json: { answer: '<img src=x onerror=alert(1)>', has_evidence: true,
    sources: [{ article_id: 18, title: '<script>danger</script>', version_no: 5, chunk_index: 0, heading_path: [{ level: 2, text: '<b>heading</b>' }] }] } }))
  await ask(page)
  await expect(page.getByRole('region', { name: '知识回答' }).locator('img, script, b')).toHaveCount(0)
  await expect(page.getByRole('link', { name: '<script>danger</script>' })).toHaveAttribute('href', '/article/18')
  await page.unroute('**/knowledge/chat')
  await page.route('**/knowledge/chat', route => route.fulfill({ status: 502, json: { detail: 'unavailable' } }))
  await page.getByLabel('你的问题').fill('重试')
  await page.getByRole('button', { name: '提问', exact: true }).click()
  await expect(page.locator('.knowledge-page [role=alert]')).toBeVisible()
  await expect(page.getByRole('region', { name: '回答来源' })).toHaveCount(0)
})

test('知识同步失败不改变成功发布结果，支持手动重试', async ({ page, request }) => {
  const headers = await login(page, request)
  const created = await request.post(`${go}/articles/drafts`, { headers, data: { title: '同步失败隔离', content: '<p>业务状态独立</p>' } })
  const article = (await created.json()).data
  await page.goto(`/article/edit/${article.id}`)
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  await page.route(`**/knowledge/sync/${article.id}`, route => route.fulfill({ status: 502, json: { detail: 'index offline' } }))
  await page.getByRole('button', { name: '发布工作版本', exact: true }).click()
  await page.getByRole('dialog', { name: '确认发布' }).getByRole('button', { name: '确认', exact: true }).click()
  await expect(page.getByTestId('article-version')).toContainText('公开版本 V1')
  await expect(page.getByText(`文章 #${article.id} 的业务操作已完成，知识索引同步失败。`)).toBeVisible()
  const persisted = (await (await request.get(`${go}/user/articles/${article.id}`, { headers })).json()).data
  expect([persisted.status, persisted.version, persisted.published_version]).toEqual(['published', 1, 1])
  await page.unroute(`**/knowledge/sync/${article.id}`)
  const retried = page.waitForResponse(r => r.url().endsWith(`/knowledge/sync/${article.id}`))
  await page.getByRole('button', { name: '重试知识同步' }).click()
  expect((await retried).status()).toBe(200)
  await expect(page.getByRole('button', { name: '重试知识同步' })).toHaveCount(0)
})

test('真实服务端按登录身份共享限流：Agent 与 Knowledge 返回 429，health 不受限', async ({ page, request }) => {
  const headers = await login(page, request)
  const created = await request.post(`${go}/articles/drafts`, { headers, data: { title: '限流工作区', content: '<p>测试</p>' } })
  const article = (await created.json()).data
  expect((await request.post(`${agent}/test/rate-limit/2`)).status()).toBe(200)
  try {
    const chat = await request.post(`${agent}/api/v1/agent/chat`, { headers, data: {
      message: '你好', mode: 'question', workspace: { article_id: article.id, version_no: 1 }
    } })
    expect(chat.status()).toBe(200)
    await page.goto('/knowledge')
    await ask(page)
    const response = page.waitForResponse(r => r.url().endsWith('/knowledge/chat'))
    await page.getByRole('button', { name: '提问', exact: true }).click()
    const limited = await response
    expect(limited.status()).toBe(429)
    expect((await limited.json()).detail.code).toBe('rate_limit_exceeded')
    expect(Number(limited.headers()['retry-after'])).toBeGreaterThan(0)
    await expect(page.locator('.knowledge-page [role=alert]')).toBeVisible()
    const alsoLimited = await request.post(`${agent}/api/v1/agent/chat`, { headers, data: {
      message: '你好', workspace: { article_id: article.id, version_no: 1 }
    } })
    expect(alsoLimited.status()).toBe(429)
    expect((await request.get(`${agent}/health`)).status()).toBe(200)
  } finally {
    await request.post(`${agent}/test/rate-limit/200`)
  }
})
