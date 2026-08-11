<script setup>
import { ref, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { getProfile, updateProfile, changePassword } from '@/api/user'
import { ElMessage } from 'element-plus'

const authStore = useAuthStore()

const profileForm = ref({
  username: '',
  email: '',
  avatar: '',
  bio: ''
})

const passwordForm = ref({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const loading = ref(false)
const updatingProfile = ref(false)
const changingPassword = ref(false)
const activeTab = ref('profile')

// 获取个人信息
async function fetchProfile() {
  loading.value = true
  try {
    const res = await getProfile()
    const user = res.data
    profileForm.value = {
      username: user.username,
      email: user.email,
      avatar: user.avatar || '',
      bio: user.bio || ''
    }
  } catch (error) {
    console.error('获取个人信息失败:', error)
  } finally {
    loading.value = false
  }
}

// 更新个人信息
async function handleUpdateProfile() {
  updatingProfile.value = true
  try {
    await updateProfile({
      avatar: profileForm.value.avatar,
      bio: profileForm.value.bio
    })
    ElMessage.success('更新成功')
    authStore.updateUser({
      avatar: profileForm.value.avatar,
      bio: profileForm.value.bio
    })
  } catch (error) {
    console.error('更新失败:', error)
  } finally {
    updatingProfile.value = false
  }
}

// 修改密码
async function handleChangePassword() {
  const { old_password, new_password, confirm_password } = passwordForm.value

  if (!old_password || !new_password || !confirm_password) {
    ElMessage.warning('请填写完整信息')
    return
  }

  if (new_password !== confirm_password) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }

  if (new_password.length < 6) {
    ElMessage.warning('新密码长度不能少于6位')
    return
  }

  changingPassword.value = true
  try {
    await changePassword({ old_password, new_password })
    ElMessage.success('密码修改成功')
    passwordForm.value = { old_password: '', new_password: '', confirm_password: '' }
  } catch (error) {
    console.error('修改密码失败:', error)
  } finally {
    changingPassword.value = false
  }
}

onMounted(() => {
  fetchProfile()
})
</script>

<template>
  <div v-loading="loading" class="profile-page container">
    <div class="page-header">
      <h1 class="page-title">个人中心</h1>
    </div>

    <el-tabs v-model="activeTab" class="profile-tabs">
      <!-- 个人信息 -->
      <el-tab-pane label="个人信息" name="profile">
        <div class="card">
          <el-form :model="profileForm" label-width="100px">
            <el-form-item label="用户名">
              <el-input :value="profileForm.username" disabled />
            </el-form-item>

            <el-form-item label="邮箱">
              <el-input :value="profileForm.email" disabled />
            </el-form-item>

            <el-form-item label="头像URL">
              <el-input
                v-model="profileForm.avatar"
                placeholder="请输入头像图片URL"
              />
              <div v-if="profileForm.avatar" class="avatar-preview">
                <el-avatar :size="80" :src="profileForm.avatar">
                  {{ profileForm.username?.charAt(0)?.toUpperCase() }}
                </el-avatar>
              </div>
            </el-form-item>

            <el-form-item label="个人简介">
              <el-input
                v-model="profileForm.bio"
                type="textarea"
                :rows="4"
                placeholder="介绍一下自己吧..."
                maxlength="200"
                show-word-limit
              />
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="updatingProfile"
                @click="handleUpdateProfile"
              >
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>

      <!-- 修改密码 -->
      <el-tab-pane label="修改密码" name="password">
        <div class="card">
          <el-form :model="passwordForm" label-width="100px" style="max-width: 500px">
            <el-form-item label="当前密码" required>
              <el-input
                v-model="passwordForm.old_password"
                type="password"
                placeholder="请输入当前密码"
                show-password
              />
            </el-form-item>

            <el-form-item label="新密码" required>
              <el-input
                v-model="passwordForm.new_password"
                type="password"
                placeholder="请输入新密码（至少6位）"
                show-password
              />
            </el-form-item>

            <el-form-item label="确认密码" required>
              <el-input
                v-model="passwordForm.confirm_password"
                type="password"
                placeholder="请再次输入新密码"
                show-password
                @keyup.enter="handleChangePassword"
              />
            </el-form-item>

            <el-form-item>
              <el-button
                type="primary"
                :loading="changingPassword"
                @click="handleChangePassword"
              >
                修改密码
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style lang="scss" scoped>
.profile-page {
  padding-top: 20px;
  padding-bottom: 40px;
  max-width: 800px;
}

.profile-tabs {
  :deep(.el-tabs__header) {
    margin-bottom: 20px;
  }
}

.avatar-preview {
  margin-top: 12px;
}
</style>
