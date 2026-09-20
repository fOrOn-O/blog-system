import test from 'node:test'
import assert from 'node:assert/strict'
import { commitThenSync, sourcePath, workspaceRequest } from './knowledge.js'
import { resolveRouteAccess } from '../router/access-control.js'

test('workspace defaults to retrieval-first and never transmits editor HTML or user identity', () => {
  const workspace = { article_id: 18, version_no: 5, content: '<p>unsaved</p>', user_id: 99, token: 'secret' }
  assert.deepEqual(workspaceRequest('question', workspace), { message: 'question', mode: 'question', workspace: { article_id: 18, version_no: 5 } })
  assert.equal(workspaceRequest('rewrite', workspace, 'write').mode, 'write')
  assert.throws(() => workspaceRequest('q', workspace, 'apply'))
})

test('public source navigation derives only from validated numeric identity', () => {
  assert.equal(sourcePath({ article_id: 18, url: 'javascript:alert(1)' }), '/article/18')
  for (const article_id of ['18', -1, 0, 'javascript:alert(1)']) assert.equal(sourcePath({ article_id }), null)
})

test('publish completes before scheduling sync and scheduling failure does not reject publish', async () => {
  const events = []
  const result = { data: { id: 18, published_version: 7 } }
  assert.equal(await commitThenSync(async () => { events.push('commit'); return result }, id => { events.push(id); throw new Error('offline') }), result)
  assert.deepEqual(events, ['commit', 18])
  await assert.rejects(commitThenSync(async () => { throw new Error('409') }, () => events.push('should not sync'), 18))
  assert.deepEqual(events, ['commit', 18])
})

test('/knowledge requires login but no admin role', () => {
  const route = { fullPath: '/knowledge', meta: { requiresAuth: true } }
  assert.deepEqual(resolveRouteAccess(route, { authenticated: false }), { name: 'Login', query: { redirect: '/knowledge' } })
  assert.equal(resolveRouteAccess(route, { authenticated: true, user: { role: 'user' } }), null)
})
