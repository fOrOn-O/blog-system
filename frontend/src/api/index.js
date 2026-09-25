import axios from 'axios'
import { getToken } from '@/utils/auth'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { getApiErrorFeedback } from './error-feedback'
import { assistantErrorMessage } from './assistant-error'

// 创建 Axios 实例
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    if (error.response) {
      const { status, data } = error.response

      const feedback = getApiErrorFeedback({
        status,
        data,
        url: error.config?.url
      })

      const assistantRequest = /\/(chat|knowledge\/sync\/\d+)$/.test((error.config?.url || '').split('?')[0])
      // 文章加载页面可接管错误展示；登录清理与跳转仍执行，Promise 仍 reject。
      if (error.config?.notifyError !== false) ElMessage.error(assistantRequest ? assistantErrorMessage(error) : feedback.message)

      if (feedback.clearSession) {
        useAuthStore().logout()
      }

      if (feedback.redirectToLogin && router.currentRoute.value.name !== 'Login') {
        router.push({
          name: 'Login',
          query: { redirect: router.currentRoute.value.fullPath }
        })
      }
    } else if (error.request) {
      if (error.config?.notifyError !== false) ElMessage.error('网络错误，请检查网络连接')
    } else {
      if (error.config?.notifyError !== false) ElMessage.error('请求配置错误')
    }

    return Promise.reject(error)
  }
)

export default api
