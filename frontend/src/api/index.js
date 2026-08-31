import axios from 'axios'
import { getToken, clearAuth } from '@/utils/auth'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { getApiErrorFeedback } from './error-feedback'

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

      ElMessage.error(feedback.message)

      if (feedback.clearSession) {
        clearAuth()
      }

      if (feedback.redirectToLogin && router.currentRoute.value.name !== 'Login') {
        router.push({
          name: 'Login',
          query: { redirect: router.currentRoute.value.fullPath }
        })
      }
    } else if (error.request) {
      ElMessage.error('网络错误，请检查网络连接')
    } else {
      ElMessage.error('请求配置错误')
    }

    return Promise.reject(error)
  }
)

export default api
