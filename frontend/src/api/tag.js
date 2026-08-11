import api from './index'

// 获取所有标签
export function getTags() {
  return api.get('/tags')
}

// 获取单个标签
export function getTag(id) {
  return api.get(`/tags/${id}`)
}

// 创建标签（管理员）
export function createTag(data) {
  return api.post('/admin/tags', data)
}

// 更新标签（管理员）
export function updateTag(id, data) {
  return api.put(`/admin/tags/${id}`, data)
}

// 删除标签（管理员）
export function deleteTag(id) {
  return api.delete(`/admin/tags/${id}`)
}
