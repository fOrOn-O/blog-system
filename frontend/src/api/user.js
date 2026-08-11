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
