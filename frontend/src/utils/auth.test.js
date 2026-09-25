import test, { beforeEach, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import { getToken, getUser, isAuthenticated, setToken, setUser } from './auth.js'

const now = 1800000000
const originalStorage = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')

beforeEach(t => {
  const values = new Map()
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: {
    getItem: key => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: key => values.delete(key)
  } })
  t.mock.method(Date, 'now', () => now * 1000)
  setUser({ username: '上次登录用户' })
})

afterEach(() => {
  if (originalStorage) Object.defineProperty(globalThis, 'localStorage', originalStorage)
  else delete globalThis.localStorage
})

function token(payload) {
  return `header.${Buffer.from(JSON.stringify(payload)).toString('base64url')}.signature`
}

test('未过期的 Base64URL JWT 保留 token 和用户', () => {
  const value = token({ exp: now + 60, username: '测试用户\uffff' })
  setToken(value)
  assert.equal(isAuthenticated(), true)
  assert.equal(getToken(), value)
  assert.deepEqual(getUser(), { username: '上次登录用户' })
})

test('已过期及恰好到期的 JWT 清除 token 和用户', () => {
  for (const exp of [now - 1, now]) {
    setToken(token({ exp }))
    setUser({ username: '旧用户' })
    assert.equal(isAuthenticated(), false)
    assert.equal(getToken(), null)
    assert.equal(getUser(), null)
  }
})

test('损坏或缺少有效 exp 的 JWT 清除本地会话', () => {
  for (const value of ['invalid', 'header.%%%.signature', token({}), token({ exp: String(now + 60) }), token({ exp: null })]) {
    setToken(value)
    setUser({ username: '旧用户' })
    assert.equal(isAuthenticated(), false)
    assert.equal(getToken(), null)
    assert.equal(getUser(), null)
  }
})

test('没有 token 时清除残留的旧用户', () => {
  assert.equal(isAuthenticated(), false)
  assert.equal(getUser(), null)
})
