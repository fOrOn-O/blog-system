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

// 作者读取最新工作版本，不增加浏览量。
export function getOwnedArticle(id) {
  return api.get(`/user/articles/${id}`)
}

export const getArticleVersions = (id, page = 1) => api.get(`/user/articles/${id}/versions`, { params: { page, limit: 20 } })
export const getArticleVersion = (id, version) => api.get(`/agent/articles/${id}/versions/${version}`)
export const getVersionDiff = (id, from, to) => api.get(`/agent/articles/${id}/diff`, { params: { from_version: from, to_version: to } })
export const createDraft = data => api.post('/articles/drafts', data)
export const saveDraft = (id, data) => api.put(`/articles/${id}/draft`, data)
export const publishArticle = (id, version) => api.post(`/articles/${id}/publish`, { expected_version: version })
export const archiveArticle = id => api.post(`/articles/${id}/archive`)

// 只投影允许的字段，批准操作直接到 Go，不经过 Agent。
const proposalBody = proposal => ({ base_version_no: proposal.base_version_no, proposed_content: proposal.proposed_content })
export const previewEditProposal = proposal => api.post(`/articles/${proposal.article_id}/edit-proposal/preview`, proposalBody(proposal))
export const applyEditProposal = proposal => api.post(`/articles/${proposal.article_id}/edit-proposal/apply`, proposalBody(proposal))

export function getMyArticles(params) {
  return api.get('/user/articles', { params })
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
