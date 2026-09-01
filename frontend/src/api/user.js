import api from './index'

// 获取个人信息
export function getProfile() {
  return api.get('/user/profile')
}

// 更新个人信息
export function updateProfile(data) {
  return api.put('/user/profile', data)
}

// 修改密码
export function changePassword(data) {
  return api.put('/user/password', data)
}

// 获取用户列表（管理员）
export function getUsers(params) {
  return api.get('/admin/users', { params })
}

// 封禁或解封用户（管理员）
export function updateUserStatus(id, isActive) {
  return api.put(`/admin/users/${id}/status`, { is_active: isActive })
}

// 重置用户密码（管理员）
export function resetUserPassword(id, newPassword) {
  return api.put(`/admin/users/${id}/password`, { new_password: newPassword })
}
