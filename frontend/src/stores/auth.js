import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getToken, setToken, clearAuth, getUser, setUser } from '@/utils/auth'
import { login as loginApi, register as registerApi } from '@/api/auth'
import { getProfile } from '@/api/user'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(getToken())
  const user = ref(getUser())

  const isAuthenticated = computed(() => !!token.value)
  const currentUser = computed(() => user.value)

  // 登录
  async function login(credentials) {
    const res = await loginApi(credentials)
    // 后端返回格式: { code: 200, data: { token: "...", user: {...} } }
    const tokenValue = res.data?.token || res.token
    const userData = res.data?.user || res.user

    token.value = tokenValue
    setToken(tokenValue)

    if (userData) {
      user.value = userData
      setUser(userData)
    } else {
      await fetchUser()
    }

    return res
  }

  // 注册
  async function register(data) {
    const res = await registerApi(data)
    const tokenValue = res.data?.token || res.token
    const userData = res.data?.user || res.user

    token.value = tokenValue
    setToken(tokenValue)

    if (userData) {
      user.value = userData
      setUser(userData)
    } else {
      await fetchUser()
    }

    return res
  }

  // 获取用户信息
  async function fetchUser() {
    try {
      const res = await getProfile()
      user.value = res.data
      setUser(res.data)
    } catch (error) {
      console.error('获取用户信息失败:', error)
    }
  }

  // 退出登录
  function logout() {
    token.value = null
    user.value = null
    clearAuth()
  }

  // 更新用户信息
  function updateUser(userData) {
    user.value = { ...user.value, ...userData }
    setUser(user.value)
  }

  return {
    token,
    user,
    isAuthenticated,
    currentUser,
    login,
    register,
    fetchUser,
    logout,
    updateUser
  }
})
