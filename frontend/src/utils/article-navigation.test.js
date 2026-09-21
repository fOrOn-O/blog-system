import test from 'node:test'
import assert from 'node:assert/strict'
import { myArticlePath, articleLoadError } from './article-navigation.js'

test('我的文章按公开状态选择路径，不将草稿或归档导向公开详情', () => {
  for (const [status, published_version] of [['draft', 0], ['archived', 2], ['published', 0]]) {
    assert.equal(myArticlePath({ id: 18, status, published_version }), '/article/edit/18')
  }
  assert.equal(myArticlePath({ id: 18, status: 'published', published_version: 2, version: 3 }), '/article/18')
})

test('文章加载失败区分身份、权限、缺失和服务异常，不转发内部信息', () => {
  for (const [status, expected] of [[401, /登录已失效/], [403, /没有权限/], [404, /不存在/], [502, /加载失败/]]) {
    const message = articleLoadError({ response: { status, data: { message: 'private-backend' } } })
    assert.match(message, expected)
    assert.ok(!message.includes('private-backend'))
  }
})
