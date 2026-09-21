export function myArticlePath(article) {
  return article.status === 'published' && article.published_version > 0
    ? `/article/${article.id}` : `/article/edit/${article.id}`
}

export function articleLoadError(error) {
  switch (error?.response?.status) {
    case 401: return '登录已失效，请重新登录。'
    case 403: return '你没有权限查看这篇文章。'
    case 404: return '文章不存在或当前不可访问。'
    default: return '文章或版本加载失败，请重试。已完成的应用不会自动重试。'
  }
}
