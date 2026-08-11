import test from 'node:test'
import assert from 'node:assert/strict'

import { getApiErrorFeedback } from './error-feedback.js'

test('invalid login keeps the server message and does not clear the session', () => {
  assert.deepEqual(
    getApiErrorFeedback({
      status: 401,
      data: { message: '用户名或密码错误' },
      url: '/auth/login'
    }),
    {
      message: '用户名或密码错误',
      clearSession: false,
      redirectToLogin: false
    }
  )
})

test('disabled account keeps the server message', () => {
  assert.deepEqual(
    getApiErrorFeedback({
      status: 401,
      data: { message: '账号已被禁用' },
      url: '/auth/login'
    }),
    {
      message: '账号已被禁用',
      clearSession: false,
      redirectToLogin: false
    }
  )
})

test('protected request with an invalid token clears the expired session', () => {
  assert.deepEqual(
    getApiErrorFeedback({
      status: 401,
      data: { message: '无效的token' },
      url: '/user/profile'
    }),
    {
      message: '登录已过期，请重新登录',
      clearSession: true,
      redirectToLogin: true
    }
  )
})
