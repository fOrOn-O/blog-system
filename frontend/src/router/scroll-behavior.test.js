import test from 'node:test'
import assert from 'node:assert/strict'

import { getScrollTarget, NAV_SCROLL_OFFSET, scrollBehavior } from './scroll-behavior.js'

test('fresh route navigation starts at the top', () => {
  assert.deepEqual(getScrollTarget({ hash: '' }), { top: 0 })
})

test('home section links scroll below the sticky navigation', () => {
  assert.deepEqual(getScrollTarget({ hash: '#featured' }), {
    el: '#featured',
    top: NAV_SCROLL_OFFSET,
    behavior: 'smooth'
  })
})

test('browser back restores the exact saved position', async () => {
  const previousWindow = globalThis.window
  globalThis.window = { setTimeout: (callback) => callback() }

  try {
    const savedPosition = { left: 0, top: 1480 }
    assert.deepEqual(await scrollBehavior({ hash: '' }, {}, savedPosition), savedPosition)
  } finally {
    globalThis.window = previousWindow
  }
})

test('changing only the home page does not force a scroll reset', () => {
  assert.equal(
    getScrollTarget(
      { name: 'Home', hash: '', query: { page: '2' } },
      { name: 'Home', hash: '', query: {} }
    ),
    false
  )
})
