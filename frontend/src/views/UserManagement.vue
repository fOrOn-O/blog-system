<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { getUsers, resetUserPassword, updateUserStatus } from '@/api/user'
import { ElMessage, ElMessageBox } from 'element-plus'

const users = ref([])
const loading = ref(false)
const page = ref(1)
const limit = 10
const total = ref(0)
const statusSubmittingId = ref(null)

const passwordDialogVisible = ref(false)
const passwordSubmitting = ref(false)
const passwordTarget = ref(null)
const newPasswordInput = ref(null)
const passwordForm = ref({
  new_password: '',
  confirm_password: ''
})

const passwordDialogTitle = computed(() => (
  `重置用户“${passwordTarget.value?.username || ''}”的密码`
))

const canSubmitPassword = computed(() => {
  const { new_password: newPassword, confirm_password: confirmPassword } = passwordForm.value
  return newPassword.length >= 6 &&
    newPassword.length <= 72 &&
    newPassword === confirmPassword &&
    !passwordSubmitting.value
})

async function fetchUsers() {
  loading.value = true
  try {
    const response = await getUsers({ page: page.value, limit })
    users.value = Array.isArray(response.data) ? response.data : []
    total.value = response.meta?.total ?? users.value.length
  } catch (error) {
    console.error('获取用户列表失败:', error)
    users.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function formatDate(dateString) {
  if (!dateString) return '—'
  return new Date(dateString).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  })
}

async function handleStatusChange(user) {
  if (user.role === 'admin') return

  const willEnable = !user.is_active
  const action = willEnable ? '解封' : '封禁'
  const message = willEnable
    ? `确定要解封用户“${user.username}”吗？解封后该用户可以重新登录。`
    : `确定要封禁用户“${user.username}”吗？封禁后该用户将无法继续访问需要登录的功能。`

  try {
    await ElMessageBox.confirm(message, `${action}用户`, {
      confirmButtonText: action,
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: willEnable ? '' : 'el-button--danger'
    })

    statusSubmittingId.value = user.id
    await updateUserStatus(user.id, willEnable)
    ElMessage.success(willEnable ? '用户已解封' : '用户已封禁')
    await fetchUsers()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error(`${action}用户失败`)
    }
  } finally {
    statusSubmittingId.value = null
  }
}

function clearPasswordForm() {
  passwordForm.value = { new_password: '', confirm_password: '' }
  passwordTarget.value = null
}

function openPasswordDialog(user) {
  if (user.role === 'admin') return

  clearPasswordForm()
  passwordTarget.value = user
  passwordDialogVisible.value = true
}

function closePasswordDialog() {
  if (passwordSubmitting.value) return
  passwordDialogVisible.value = false
}

function focusPasswordInput() {
  nextTick(() => newPasswordInput.value?.focus())
}

async function handleResetPassword() {
  const { new_password: newPassword, confirm_password: confirmPassword } = passwordForm.value

  if (newPassword.length < 6 || newPassword.length > 72) {
    ElMessage.warning('新密码长度必须为 6–72 个字符')
    return
  }
  if (newPassword !== confirmPassword) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  if (!passwordTarget.value || passwordTarget.value.role === 'admin') return

  passwordSubmitting.value = true
  try {
    await resetUserPassword(passwordTarget.value.id, newPassword)
    ElMessage.success('密码重置成功')
    passwordForm.value = { new_password: '', confirm_password: '' }
    passwordDialogVisible.value = false
  } catch {
    // 统一请求拦截器负责展示服务端错误；这里不记录可能包含密码的请求对象。
  } finally {
    passwordSubmitting.value = false
  }
}

function handlePageChange(nextPage) {
  page.value = nextPage
  fetchUsers()
}

onMounted(fetchUsers)
</script>

<template>
  <section class="user-admin-page container animate-in">
    <header class="workspace-header">
      <div>
        <span class="workspace-kicker">ADMIN / USERS</span>
        <h1>用户管理</h1>
        <p>查看账号状态，并管理普通用户的访问权限与登录密码。</p>
      </div>
      <div class="user-total" aria-live="polite">
        <strong>{{ total }}</strong>
        <span>位用户</span>
      </div>
    </header>

    <div v-loading="loading" class="user-table-wrap" :aria-busy="loading">
      <el-table :data="users" row-key="id" empty-text="暂无用户" class="user-table">
        <el-table-column prop="id" label="ID" width="78">
          <template #default="{ row }">
            <span class="user-id">#{{ row.id }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="username" label="用户名" min-width="145">
          <template #default="{ row }">
            <span class="username">{{ row.username }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="email" label="邮箱" min-width="220" />

        <el-table-column prop="role" label="角色" width="105">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'warning' : 'info'" effect="plain" round>
              {{ row.role === 'admin' ? '管理员' : '普通用户' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="is_active" label="状态" width="100">
          <template #default="{ row }">
            <span class="status" :class="row.is_active ? 'is-active' : 'is-disabled'">
              <i aria-hidden="true"></i>
              {{ row.is_active ? '正常' : '已封禁' }}
            </span>
          </template>
        </el-table-column>

        <el-table-column prop="created_at" label="注册时间" width="130">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>

        <el-table-column label="操作" width="230" align="right" fixed="right">
          <template #default="{ row }">
            <div v-if="row.role !== 'admin'" class="row-actions">
              <el-button
                text
                :type="row.is_active ? 'danger' : 'success'"
                :loading="statusSubmittingId === row.id"
                @click="handleStatusChange(row)"
              >
                {{ row.is_active ? '封禁' : '解封' }}
              </el-button>
              <el-button text type="primary" @click="openPasswordDialog(row)">重置密码</el-button>
            </div>
            <span v-else class="admin-note">管理员账号</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-if="total > limit" class="pagination-wrap">
      <el-pagination
        background
        layout="prev, pager, next"
        :current-page="page"
        :page-size="limit"
        :total="total"
        @current-change="handlePageChange"
      />
    </div>

    <el-dialog
      v-model="passwordDialogVisible"
      :title="passwordDialogTitle"
      width="min(92vw, 460px)"
      :show-close="!passwordSubmitting"
      :close-on-click-modal="!passwordSubmitting"
      :close-on-press-escape="!passwordSubmitting"
      @opened="focusPasswordInput"
      @closed="clearPasswordForm"
    >
      <p class="dialog-help">新密码保存后，用户需要使用新密码再次登录。</p>
      <el-form label-position="top" @submit.prevent="handleResetPassword">
        <el-form-item label="新密码">
          <el-input
            ref="newPasswordInput"
            v-model="passwordForm.new_password"
            type="password"
            show-password
            maxlength="72"
            autocomplete="new-password"
            placeholder="请输入 6–72 个字符"
          />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input
            v-model="passwordForm.confirm_password"
            type="password"
            show-password
            maxlength="72"
            autocomplete="new-password"
            placeholder="请再次输入新密码"
            @keyup.enter="handleResetPassword"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button :disabled="passwordSubmitting" @click="closePasswordDialog">取消</el-button>
        <el-button type="primary" :loading="passwordSubmitting" :disabled="!canSubmitPassword" @click="handleResetPassword">
          确认重置
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style lang="scss" scoped>
.user-admin-page {
  max-width: 1180px;
  padding-top: 18px;
  padding-bottom: 44px;
}

.workspace-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 32px;
  padding-bottom: 30px;
  border-bottom: 1px solid rgba(25, 50, 65, 0.16);
}

.workspace-kicker {
  display: block;
  margin-bottom: 12px;
  color: #a66d2e;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

h1 {
  margin: 0;
  color: #102332;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: clamp(34px, 5vw, 52px);
  font-weight: 600;
  letter-spacing: -0.04em;
  line-height: 1.08;
}

.workspace-header p {
  max-width: 620px;
  margin: 13px 0 0;
  color: #5e7180;
  font-size: 14px;
}

.user-total {
  display: flex;
  align-items: baseline;
  gap: 8px;
  color: #71808b;
  white-space: nowrap;
}

.user-total strong {
  color: #17384a;
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 38px;
  font-weight: 500;
  line-height: 1;
}

.user-total span {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
}

.user-table-wrap {
  min-height: 330px;
  margin-top: 28px;
  overflow-x: auto;
  border-top: 2px solid #17384a;
  border-bottom: 1px solid rgba(25, 50, 65, 0.16);
  background: rgba(255, 255, 255, 0.48);
}

.user-table {
  min-width: 980px;
  --el-table-border-color: rgba(25, 50, 65, 0.1);
  --el-table-header-bg-color: rgba(241, 239, 233, 0.82);
  --el-table-row-hover-bg-color: rgba(228, 182, 120, 0.09);
  --el-table-bg-color: transparent;
  --el-fill-color-lighter: rgba(241, 239, 233, 0.82);
}

:deep(.el-table th.el-table__cell) {
  height: 46px;
  color: #73818b;
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.1em;
}

:deep(.el-table td.el-table__cell) {
  height: 61px;
  color: #405662;
  font-size: 13px;
}

.user-id {
  color: #81909a;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
}

.username {
  color: #17384a;
  font-weight: 700;
}

.status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  font-weight: 700;
}

.status i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.status.is-active {
  color: #2f7d61;
}

.status.is-disabled {
  color: #b04455;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
}

.admin-note {
  color: #8a949b;
  font-size: 12px;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  padding-top: 24px;
}

.dialog-help {
  margin: -4px 0 20px;
  color: #6d7c86;
  font-size: 13px;
  line-height: 1.7;
}

:deep(.el-dialog) {
  border-radius: 16px;
  overflow: hidden;
}

:deep(.el-dialog__title) {
  color: #17384a;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: 20px;
  font-weight: 600;
}

@media (max-width: 680px) {
  .user-admin-page {
    padding-top: 4px;
  }

  .workspace-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 20px;
  }

  .user-total strong {
    font-size: 32px;
  }

  .pagination-wrap {
    justify-content: center;
  }
}

@media (prefers-reduced-motion: reduce) {
  .user-admin-page,
  :deep(.el-table__body tr) {
    animation: none;
    transition: none;
  }
}
</style>
