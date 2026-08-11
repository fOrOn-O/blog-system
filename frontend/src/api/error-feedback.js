function isAuthSubmission(url = '') {
  const path = url.split('?')[0]
  return path.endsWith('/auth/login') || path.endsWith('/auth/register')
}

export function getApiErrorFeedback({ status, data, url }) {
  if (status === 401) {
    if (isAuthSubmission(url)) {
      return {
        message: data?.message || '登录失败，请检查账号和密码',
        clearSession: false,
        redirectToLogin: false
      }
    }

    return {
      message: '登录已过期，请重新登录',
      clearSession: true,
      redirectToLogin: true
    }
  }

  const messages = {
    403: '没有权限访问',
    404: '请求的资源不存在',
    422: data?.message || '请求参数错误',
    429: '请求过于频繁，请稍后再试',
    500: '服务器错误，请稍后再试'
  }

  return {
    message: messages[status] || data?.message || '请求失败',
    clearSession: false,
    redirectToLogin: false
  }
}
