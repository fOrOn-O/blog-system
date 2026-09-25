const TOKEN_KEY = 'blog_token'
const USER_KEY = 'blog_user'

// 获取 Token
export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

// 设置 Token
export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

// 移除 Token
export function removeToken() {
  localStorage.removeItem(TOKEN_KEY)
}

// 获取用户信息
export function getUser() {
  const user = localStorage.getItem(USER_KEY)
  return user ? JSON.parse(user) : null
}

// 设置用户信息
export function setUser(user) {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

// 移除用户信息
export function removeUser() {
  localStorage.removeItem(USER_KEY)
}

// 清除所有认证信息
export function clearAuth() {
  removeToken()
  removeUser()
}

// 检查是否已登录
export function isAuthenticated() {
  const parts = getToken()?.split('.')
  if (parts?.length === 3 && parts.every(Boolean)) {
    try {
      const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
      const bytes = Uint8Array.from(atob(base64), char => char.charCodeAt(0))
      const payload = JSON.parse(new TextDecoder().decode(bytes))
      // 此处仅判断本地会话是否过期；签名和权限仍由后端验证。
      if (Number.isFinite(payload?.exp) && payload.exp > Date.now() / 1000) {
        return true
      }
    } catch {
      // 损坏或无法解析的 Token 同样不能作为已登录状态。
    }
  }
  clearAuth()
  return false
}
