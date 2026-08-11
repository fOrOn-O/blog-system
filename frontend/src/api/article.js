import api from './index'

// 获取文章列表
export function getArticles(params) {
  return api.get('/articles', { params })
}

// 搜索文章
export function searchArticles(params) {
  return api.get('/articles/search', { params })
}

// 获取文章详情
export function getArticle(id) {
  return api.get(`/articles/${id}`)
}

// 创建文章
export function createArticle(data) {
  return api.post('/articles', data)
}

// 更新文章
export function updateArticle(id, data) {
  return api.put(`/articles/${id}`, data)
}

// 删除文章
export function deleteArticle(id) {
  return api.delete(`/articles/${id}`)
}

// 点赞文章
export function likeArticle(id) {
  return api.post(`/articles/${id}/like`)
}

// 取消点赞
export function unlikeArticle(id) {
  return api.delete(`/articles/${id}/like`)
}

// 获取点赞信息
export function getLikeInfo(id) {
  return api.get(`/articles/${id}/likes`)
}

// 获取文章评论
export function getComments(id) {
  return api.get(`/articles/${id}/comments`)
}

// 发表评论
export function createComment(articleId, data) {
  return api.post(`/articles/${articleId}/comments`, data)
}

// 删除评论
export function deleteComment(id) {
  return api.delete(`/comments/${id}`)
}

// 收藏文章
export function favoriteArticle(id) {
  return api.post(`/articles/${id}/favorite`)
}

// 取消收藏
export function unfavoriteArticle(id) {
  return api.delete(`/articles/${id}/favorite`)
}

// 检查是否已收藏
export function checkFavorited(id) {
  return api.get(`/articles/${id}/favorite`)
}

// 获取用户收藏列表
export function getFavorites(params) {
  return api.get('/user/favorites', { params })
}
