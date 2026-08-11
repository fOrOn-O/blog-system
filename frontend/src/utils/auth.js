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
  return !!getToken()
}
