import api from './index'

// 上传图片
export function uploadImage(file) {
  const formData = new FormData()
  formData.append('file', file)

  return api.post('/upload/image', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}
