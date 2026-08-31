import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveRouteAccess } from './access-control.js'

test('anonymous users are sent to login before opening admin pages', () => {
  const to = {
    fullPath: '/admin/tags',
    meta: { requiresAuth: true, requiresAdmin: true }
  }

  assert.deepEqual(
    resolveRouteAccess(to, { authenticated: false, user: null }),
    { name: 'Login', query: { redirect: '/admin/tags' } }
  )
})

test('non-admin users cannot open admin pages', () => {
  const to = {
    fullPath: '/admin/tags',
    meta: { requiresAuth: true, requiresAdmin: true }
  }

  assert.deepEqual(
    resolveRouteAccess(to, { authenticated: true, user: { role: 'user' } }),
    { name: 'Home' }
  )
})

test('admin users can open admin pages', () => {
  const to = {
    fullPath: '/admin/tags',
    meta: { requiresAuth: true, requiresAdmin: true }
  }

  assert.equal(
    resolveRouteAccess(to, { authenticated: true, user: { role: 'admin' } }),
    null
  )
})

test('authenticated users are redirected away from guest pages', () => {
  const to = {
    fullPath: '/login',
    meta: { guest: true }
  }

  assert.deepEqual(
    resolveRouteAccess(to, { authenticated: true, user: { role: 'user' } }),
    { name: 'Home' }
  )
})
