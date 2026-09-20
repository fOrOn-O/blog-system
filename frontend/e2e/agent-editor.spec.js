import { test, expect } from '@playwright/test'
import { mkdir } from 'node:fs/promises'

const go = 'http://127.0.0.1:18080/api/v1'

async function openArticle(page, request, versions = 1) {
  const login = await request.post(`${go}/auth/login`, { data: { username: 'user0', password: 'e2e-test-password' } })
  expect(login.ok()).toBeTruthy()
  const { token, user } = (await login.json()).data
  const headers = { Authorization: `Bearer ${token}` }
  const created = await request.post(`${go}/articles`, { headers, data: { title: 'Agent E2E 文章', content: '<h2>原始标题</h2><p>原始正文</p>' } })
  expect(created.status()).toBe(201)
  let article = (await created.json()).data
  if (versions === 2) {
    const res = await request.put(`${go}/articles/${article.id}/draft`, { headers, data: { expected_version: 1, content: '<h2>原始标题</h2><p>第二版正文</p>' } })
    article = (await res.json()).data
  }
  await page.addInitScript(({ token, user }) => {
    localStorage.setItem('blog_token', token)
    localStorage.setItem('blog_user', JSON.stringify(user))
  }, { token, user })
  await page.goto(`/article/edit/${article.id}`)
  await expect(page.getByTestId('agent-workspace')).toContainText(`V${versions}`)
  return { article, headers }
}

async function send(page, text = '请补充结论') {
  await page.getByLabel('助手模式').selectOption(text === '你好' ? 'question' : 'write')
  await page.getByLabel('向助手提问').fill(text)
  const response = page.waitForResponse(r => r.url().endsWith('/chat'))
  await page.getByRole('button', { name: '发送', exact: true }).click()
  expect((await response).status()).toBe(200)
}

async function proposal(page) {
  await send(page)
  await expect(page.getByRole('region', { name: '编辑提案' })).toBeVisible()
}

async function preview(page) {
  await page.getByRole('button', { name: '预览修改', exact: true }).click()
  await expect(page.getByRole('region', { name: '正文差异' })).toBeVisible()
}

async function approve(page) {
  await page.getByRole('button', { name: '应用修改', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '确认应用修改' })
  await expect(dialog).toContainText('不会自动发布')
  await dialog.getByRole('button', { name: '确认应用', exact: true }).click()
}

test('真实 Vue → Python Graph → Go：问答、提案、预览、人工确认、草稿刷新', async ({ page, request }) => {
  const { article, headers } = await openArticle(page, request)
  await send(page, '你好')
  await expect(page.getByText(/没有足够检索依据/)).toBeVisible()
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)
  const requests = []
  page.on('request', req => requests.push(req))
  await proposal(page)
  expect(requests.find(r => r.url().endsWith('/chat')).postDataJSON()).toEqual({ message: '请补充结论', mode: 'write', workspace: { article_id: article.id, version_no: 1 } })
  await expect(page.getByText('保留原文，补充结论段', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  requests.length = 0
  await preview(page)
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeEnabled()
  await mkdir('../output/playwright', { recursive: true })
  await page.screenshot({ path: '../output/playwright/task13-proposal.png', fullPage: true })
  expect(requests).toHaveLength(1)
  expect(requests[0].url()).toContain(`/articles/${article.id}/edit-proposal/preview`)
  expect(requests[0].postDataJSON()).toEqual({ base_version_no: 1, proposed_content: article.content + '<p>助手补充的结论。</p>' })
  await page.getByRole('button', { name: '应用修改', exact: true }).click()
  await expect(page.getByRole('dialog')).toContainText('不会自动发布')
  expect(requests.filter(r => r.url().endsWith('/apply'))).toHaveLength(0)
  await page.getByRole('dialog').getByRole('button', { name: '取消', exact: true }).click()
  let release
  const wait = new Promise(resolve => { release = resolve })
  await page.route('**/edit-proposal/apply', async route => { await wait; await route.continue() })
  await approve(page)
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: '应用修改', exact: true }).click({ force: true })
  expect(requests.filter(r => r.url().endsWith('/apply'))).toHaveLength(1)
  release()
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByTestId('article-version')).toContainText('公开版本 V1')
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)
  await expect(page.getByRole('region', { name: '正文差异' })).toHaveCount(0)
  await expect(page.getByLabel('文章正文编辑器')).toContainText('助手补充的结论。')
  expect(await page.getByLabel('活动版本').locator('option').allTextContents()).toEqual(['V2', 'V1'])
  expect(requests.some(r => r.url().includes(`/user/articles/${article.id}/versions`))).toBeTruthy()
  expect(requests.some(r => /\/(publish|archive|index)(\?|$)/.test(r.url()))).toBeFalsy()
  const current = (await (await request.get(`${go}/user/articles/${article.id}`, { headers })).json()).data
  expect([current.version, current.published_version, current.status]).toEqual([2, 1, 'published'])
  const publicArticle = (await (await request.get(`${go}/articles/${article.id}`)).json()).data
  expect(publicArticle.content).toBe(article.content)
})

test('工作区切换不重绑提案；丢弃仅清本地状态；后续请求使用新版本', async ({ page, request }) => {
  const { article } = await openArticle(page, request, 2)
  await proposal(page)
  await page.getByLabel('活动版本').selectOption('1')
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  await expect(page.getByRole('heading', { name: '编辑提案 · 基于 V2' })).toBeVisible()
  await expect(page.getByText('此提案属于其他文章或版本，无法在当前工作区应用。可切回原版本或丢弃。')).toBeVisible()
  await preview(page)
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  const calls = []
  page.on('request', req => calls.push(req.url()))
  await page.getByRole('button', { name: '丢弃提案' }).click()
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)
  expect(calls).toEqual([])
  const next = page.waitForRequest(r => r.url().endsWith('/chat'))
  await send(page, '你好')
  expect((await next).postDataJSON().workspace).toEqual({ article_id: article.id, version_no: 1 })
})

test('真实版本冲突：409 后禁用 Apply，不重试或改变基础版本', async ({ page, request }) => {
  const { article, headers } = await openArticle(page, request)
  await proposal(page)
  await preview(page)
  const changed = await request.put(`${go}/articles/${article.id}/draft`, { headers, data: { expected_version: 1, content: '<p>其他窗口保存</p>' } })
  expect(changed.ok()).toBeTruthy()
  const calls = []
  page.on('request', r => { if (r.url().endsWith('/apply')) calls.push(r.postDataJSON()) })
  await approve(page)
  await expect(page.getByText('提案已过期：文章在生成提案后发生变化。请刷新文章并重新生成提案。')).toBeVisible()
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  await expect(page.getByTestId('article-version')).toContainText('当前工作版本 V2')
  await page.getByRole('button', { name: '刷新文章', exact: true }).click()
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByRole('heading', { name: '编辑提案 · 基于 V1' })).toBeVisible()
  expect(calls).toHaveLength(1)
  expect(calls[0].base_version_no).toBe(1)
  await page.getByRole('button', { name: '丢弃提案' }).click()
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)
})

test('不可信提案及差异只以文本显示；重复发送被阻止', async ({ page, request }) => {
  const { article } = await openArticle(page, request)
  let release
  const wait = new Promise(resolve => { release = resolve })
  let sends = 0
  const evil = '<img src=x onerror="window.proposalExecuted=1"><script>window.proposalExecuted=1</script>'
  await page.route('**/chat', async route => {
    sends++
    await wait
    await route.fulfill({ json: { answer: '<b>建议</b>', proposal: { article_id: article.id, base_version_no: 1, change_summary: [evil], proposed_content: evil } } })
  })
  await page.getByLabel('向助手提问').fill('测试请求')
  await page.getByRole('button', { name: '发送', exact: true }).click()
  await expect(page.getByRole('button', { name: '发送', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: '发送', exact: true }).click({ force: true })
  expect(sends).toBe(1)
  release()
  await expect(page.getByRole('region', { name: '编辑提案' })).toBeVisible()
  expect(await page.getByRole('region', { name: '编辑提案' }).locator('img,script,b').count()).toBe(0)
  await page.route('**/edit-proposal/preview', route => route.fulfill({ json: { code: 200, data: { article_id: article.id, base_version_no: 1, field_changes: {}, content: { changed: true, changes: [{ operation: 'insert', after: { type: 'paragraph', text: evil } }] } } } }))
  await preview(page)
  expect(await page.getByRole('region', { name: '正文差异' }).locator('img,script').count()).toBe(0)
  expect(await page.evaluate(() => window.proposalExecuted)).toBeUndefined()
})

test('未保存本地编辑阻止 Apply；手动草稿、历史 Diff、发布和归档回归', async ({ page, request }) => {
  await openArticle(page, request)
  await proposal(page)
  await preview(page)
  await page.getByPlaceholder('请输入文章标题', { exact: true }).fill('本地修改')
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  await expect(page.getByText(/有未保存修改/)).toBeVisible()
  await page.getByRole('button', { name: '丢弃提案' }).click()
  await page.getByLabel('文章正文编辑器').fill('手动保存正文')
  await page.getByRole('button', { name: '保存草稿', exact: true }).click()
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByTestId('article-version')).toContainText('公开版本 V1')
  await page.getByRole('button', { name: '与上一版比较' }).click()
  await expect(page.getByRole('region', { name: '正文差异' })).toContainText('手动保存正文')
  await page.getByRole('button', { name: '发布工作版本' }).click()
  await page.getByRole('dialog').getByRole('button', { name: '确认', exact: true }).click()
  await expect(page.getByTestId('article-version')).toContainText('公开版本 V2')
  await page.getByRole('button', { name: '归档文章' }).click()
  await page.getByRole('dialog').getByRole('button', { name: '确认', exact: true }).click()
  await expect(page.getByTestId('article-version')).toContainText('archived')
  await expect(page.getByRole('button', { name: '保存草稿', exact: true })).toBeDisabled()
})

test('预览失败与迟到响应不会批准被丢弃的提案', async ({ page, request }) => {
  await openArticle(page, request)
  await proposal(page)
  await page.route('**/edit-proposal/preview', route => route.fulfill({ status: 500, json: { code: 500, message: 'test failure' } }))
  await page.getByRole('button', { name: '预览修改', exact: true }).click()
  await expect(page.getByText(/预览失败，未批准/)).toBeVisible()
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  await page.unroute('**/edit-proposal/preview')
  let release
  const wait = new Promise(resolve => { release = resolve })
  await page.route('**/edit-proposal/preview', async route => { await wait; await route.continue() })
  await page.getByRole('button', { name: '预览修改', exact: true }).click()
  await page.getByRole('button', { name: '丢弃提案' }).click()
  release()
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)
  await expect(page.getByRole('region', { name: '正文差异' })).toHaveCount(0)
})

test('助手请求失败恢复发送状态，401 沿用前端登录失效流程', async ({ page, request }) => {
  await openArticle(page, request)
  await page.route('**/chat', route => route.fulfill({ status: 502, json: { detail: '助手暂时不可用' } }))
  await page.getByLabel('向助手提问').fill('你好')
  await page.getByRole('button', { name: '发送', exact: true }).click()
  await expect(page.getByText('助手请求失败，请检查登录或稍后重新发送。')).toBeVisible()
  await page.getByLabel('向助手提问').fill('重新提问')
  await expect(page.getByRole('button', { name: '发送', exact: true })).toBeEnabled()
  await page.unroute('**/chat')
  await page.route('**/chat', route => route.fulfill({ status: 401, json: { detail: '登录已过期' } }))
  await page.getByRole('button', { name: '发送', exact: true }).click()
  await expect(page).toHaveURL(/\/login\?redirect=/)
  expect(await page.evaluate(() => localStorage.getItem('blog_token'))).toBeNull()
})

test('新文章先保存草稿获得工作区，页面加载不会产生虚假的未保存修改', async ({ page, request }) => {
  await openArticle(page, request)
  await page.goto('/article/edit')
  await expect(page.getByText('先保存文章草稿，再使用助手。')).toBeVisible()
  await page.getByPlaceholder('请输入文章标题', { exact: true }).fill('新草稿')
  await page.getByLabel('文章正文编辑器').fill('新草稿正文')
  await page.getByRole('button', { name: '保存草稿', exact: true }).click()
  await expect(page).toHaveURL(/\/article\/edit\/\d+$/)
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  await expect(page.getByTestId('article-version')).toContainText('公开版本 无')
  await expect(page.getByText(/有未保存修改/)).toHaveCount(0)
})

test('切换再切回及刷新工作区均撤销旧预览，迟到预览不能恢复批准', async ({ page, request }) => {
  await openArticle(page, request, 2)
  await proposal(page)
  await preview(page)
  await page.getByLabel('活动版本').selectOption('1')
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  await page.getByLabel('活动版本').selectOption('2')
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  await expect(page.getByRole('region', { name: '正文差异' })).toHaveCount(0)
  let release
  const wait = new Promise(resolve => { release = resolve })
  await page.route('**/edit-proposal/preview', async route => { await wait; await route.continue() })
  const response = page.waitForResponse(r => r.url().endsWith('/preview'))
  await page.getByRole('button', { name: '预览修改', exact: true }).click()
  await page.getByRole('button', { name: '刷新工作版本' }).click()
  await expect(page.getByRole('button', { name: '刷新工作版本' })).toBeEnabled()
  release()
  await response
  await expect(page.getByRole('region', { name: '正文差异' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  await page.unroute('**/edit-proposal/preview')
  await preview(page)
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeEnabled()
})

test('保存与 Apply/刷新互斥，历史读取完成前不提前更新工作区', async ({ page, request }) => {
  const { article } = await openArticle(page, request)
  await proposal(page)
  await preview(page)
  await page.getByLabel('文章正文编辑器').fill('本地新正文')
  let releaseSave, releaseHistory
  const saveWait = new Promise(resolve => { releaseSave = resolve })
  const historyWait = new Promise(resolve => { releaseHistory = resolve })
  await page.route(`**/articles/${article.id}/draft`, async route => { await saveWait; await route.continue() })
  await page.route(`**/user/articles/${article.id}/versions?*`, async route => { await historyWait; await route.continue() })
  await page.getByRole('button', { name: '保存草稿', exact: true }).click()
  await expect(page.getByRole('button', { name: '刷新工作版本' })).toBeDisabled()
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  const historyRequest = page.waitForRequest(r => r.url().includes(`/user/articles/${article.id}/versions?`))
  releaseSave()
  await historyRequest
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  await expect(page.getByTestId('article-version')).toContainText('当前工作版本 V1')
  releaseHistory()
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByTestId('article-version')).toContainText('当前工作版本 V2')
  await expect(page.getByRole('heading', { name: '编辑提案 · 基于 V1' })).toBeVisible()
  await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
})

for (const kind of ['正文', '封面']) {
  test(`${kind}图片上传期间禁止 Apply、保存与工作区切换`, async ({ page, request }) => {
    await openArticle(page, request)
    await proposal(page)
    await preview(page)
    let release
    const wait = new Promise(resolve => { release = resolve })
    await page.route('**/upload/image', async route => {
      await wait
      await route.fulfill({ json: { code: 200, data: { url: '/fixture-image.png' } } })
    })
    const input = kind === '正文' ? page.locator('.rich-text-editor input[type=file]') : page.locator('#cover-input')
    await input.setInputFiles({ name: 'fixture.png', mimeType: 'image/png', buffer: Buffer.from('fixture') })
    await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
    await expect(page.getByRole('button', { name: '保存草稿', exact: true })).toBeDisabled()
    await expect(page.getByLabel('活动版本')).toBeDisabled()
    release()
    await expect(page.getByText(/有未保存修改/)).toBeVisible()
    await expect(page.getByRole('button', { name: '应用修改', exact: true })).toBeDisabled()
  })
}

test('Apply 已提交但历史刷新失败：清除提案并锁定编辑，重载统一恢复版本', async ({ page, request }) => {
  const { article, headers } = await openArticle(page, request)
  await proposal(page)
  await preview(page)
  await page.route(`**/user/articles/${article.id}/versions?*`, route => route.fulfill({ status: 500, json: { code: 500, message: 'history unavailable' } }))
  const applies = []
  page.on('request', r => { if (r.url().endsWith('/apply')) applies.push(r) })
  await approve(page)
  await expect(page.getByText(/文章或版本刷新失败，请重试/)).toBeVisible()
  await expect(page.getByRole('region', { name: '编辑提案' })).toHaveCount(0)
  await expect(page.getByRole('region', { name: '正文差异' })).toHaveCount(0)
  await expect(page.getByTestId('agent-workspace')).toContainText('V1')
  await expect(page.getByTestId('article-version')).toContainText('当前工作版本 V1')
  await expect(page.getByRole('button', { name: '保存草稿', exact: true })).toBeDisabled()
  await expect(page.getByLabel('向助手提问')).toBeDisabled()
  const current = (await (await request.get(`${go}/user/articles/${article.id}`, { headers })).json()).data
  expect([current.version, current.published_version]).toEqual([2, 1])
  await page.unroute(`**/user/articles/${article.id}/versions?*`)
  await page.getByRole('button', { name: '重新加载', exact: true }).click()
  await expect(page.getByTestId('agent-workspace')).toContainText('V2')
  await expect(page.getByTestId('article-version')).toContainText('当前工作版本 V2 · 公开版本 V1')
  await expect(page.getByLabel('文章正文编辑器')).toContainText('助手补充的结论。')
  expect(applies).toHaveLength(1)
})

test('冲突检查的迟到响应不能覆盖随后刷新的文章版本', async ({ page, request }) => {
  const { article, headers } = await openArticle(page, request)
  await proposal(page)
  await preview(page)
  await request.put(`${go}/articles/${article.id}/draft`, { headers, data: { expected_version: 1, content: '<p>第二版</p>' } })
  let release, captured
  const wait = new Promise(resolve => { release = resolve })
  const capturedWait = new Promise(resolve => { captured = resolve })
  await page.route(`**/user/articles/${article.id}`, async route => {
    const response = await route.fetch()
    captured()
    await wait
    await route.fulfill({ response })
  }, { times: 1 })
  await approve(page)
  await capturedWait
  await request.put(`${go}/articles/${article.id}/draft`, { headers, data: { expected_version: 2, content: '<p>第三版</p>' } })
  await page.getByRole('button', { name: '刷新文章', exact: true }).click()
  await expect(page.getByTestId('agent-workspace')).toContainText('V3')
  const lateResponse = page.waitForResponse(r => r.url().endsWith(`/user/articles/${article.id}`))
  release()
  await lateResponse
  await expect(page.getByTestId('article-version')).toContainText('当前工作版本 V3')
  await expect(page.getByLabel('文章正文编辑器')).toContainText('第三版')
})
