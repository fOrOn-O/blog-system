export function assistantErrorMessage(error) {
  const status = error?.response?.status
  const code = error?.response?.data?.detail?.code
  if (status === 401) return '登录已失效，请重新登录。'
  if (status === 403) return '你没有权限执行此操作。'
  if (status === 429) {
    const value = error.response.headers?.['retry-after']
    const seconds = /^\d+$/.test(String(value)) ? Number(value) : NaN
    return Number.isSafeInteger(seconds) && seconds > 0 && seconds <= 86400
      ? `请求过于频繁，请在 ${seconds} 秒后重试。`
      : '请求过于频繁，请稍后重试。'
  }
  if (status >= 400 && status < 500) return '请求无法处理，请检查输入和所选文章版本。'
  if (status === 504 || error?.code === 'ECONNABORTED' || error?.code === 'ETIMEDOUT') return '助手响应超时，请稍后重试。'
  if (status >= 500) {
    if (code === 'model_rate_limited') return '助手模型服务繁忙，请稍后重试。'
    if (code === 'proposal_not_created' || code === 'model_output_invalid') return '助手未能生成有效的完整提案，请重新发起请求；文章尚未保存。'
    return '助手服务暂时失败，请稍后重试。'
  }
  if (error?.request) return '无法连接助手服务，请检查网络后重试。'
  return '助手未完成请求，请稍后重试。'
}
