import test from 'node:test'
import assert from 'node:assert/strict'
import { normalizeCommentsResponse } from './comments.js'

test('normalizes a paginated comments response to its data array', () => {
  const comments = [{ id: 1, content: 'hello' }]
  assert.equal(normalizeCommentsResponse({ data: comments }), comments)
})

test('normalizes null and non-array API data to an empty array', () => {
  assert.deepEqual(normalizeCommentsResponse({ data: null }), [])
  assert.deepEqual(normalizeCommentsResponse({ data: { id: 1 } }), [])
  assert.deepEqual(normalizeCommentsResponse(null), [])
})

test('keeps direct array responses for backwards compatibility', () => {
  const comments = [{ id: 2, content: 'world' }]
  assert.equal(normalizeCommentsResponse(comments), comments)
})
