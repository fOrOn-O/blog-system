<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const form = ref({
  username: '',
  password: ''
})

const loading = ref(false)

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('请填写所有字段')
    return
  }

  loading.value = true
  try {
    await authStore.login(form.value)
    ElMessage.success('登录成功')
    const redirect = route.query.redirect || '/'
    router.push(redirect)
  } catch (error) {
    console.error('Login failed:', error)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-container">
      <!-- Terminal header -->
      <div class="auth-header">
        <div class="terminal-bar">
          <span class="terminal-title">$ login</span>
        </div>
        <div class="terminal-body">
          <div class="terminal-line">
            <span class="t-prompt">&gt;</span>
            <span class="t-cmd">请登录以继续...</span>
          </div>
        </div>
      </div>

      <!-- Form -->
      <div class="auth-form">
        <div class="form-group">
          <label class="form-label">用户名</label>
          <input
            v-model="form.username"
            type="text"
            class="form-input"
            placeholder="请输入用户名"
            @keyup.enter="handleLogin"
          />
        </div>

        <div class="form-group">
          <label class="form-label">密码</label>
          <input
            v-model="form.password"
            type="password"
            class="form-input"
            placeholder="请输入密码"
            @keyup.enter="handleLogin"
          />
        </div>

        <button
          class="form-submit"
          :class="{ loading }"
          :disabled="loading"
          @click="handleLogin"
        >
          <span v-if="loading" class="spinner"></span>
          <span v-else>→ 登录</span>
        </button>

        <div class="form-footer">
          <span class="footer-text">没有账号？</span>
          <a class="footer-link" @click="router.push('/register')">立即注册</a>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.auth-page {
  min-height: calc(100vh - 60px - 120px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
}

.auth-container {
  width: 100%;
  max-width: 400px;
}

// ── Terminal Header ────────────────────────────────────
.auth-header {
  background: #F0F4F8;
  border-radius: 8px 8px 0 0;
  overflow: hidden;
  border: 1px solid #E2E8F0;
  border-bottom: none;
}

.terminal-bar {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #E2E8F0;

  .terminal-title {
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    font-weight: 600;
    color: #3B68CC;
  }
}

.terminal-body {
  padding: 14px 16px;
}

.terminal-line {
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  color: #5B6B7F;
  line-height: 1.6;

  .t-prompt {
    color: #3B68CC;
    margin-right: 8px;
    font-weight: 600;
  }

  .t-cmd {
    color: #5B6B7F;
  }
}

// ── Form ───────────────────────────────────────────────
.auth-form {
  background: #FFFFFF;
  border: 1px solid #E8ECF0;
  border-top: none;
  border-radius: 0 0 8px 8px;
  padding: 28px 24px;
}

.form-group {
  margin-bottom: 20px;
}

.form-label {
  display: block;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.form-input {
  width: 100%;
  padding: 10px 14px;
  background: #F8F9FA;
  border: 1px solid #E8ECF0;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;
  color: #2D3748;
  outline: none;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;

  &::placeholder {
    color: var(--text-muted);
  }

  &:focus {
    border-color: #3B68CC;
    box-shadow: 0 0 0 3px rgba(91, 141, 239, 0.1);
  }
}

.form-submit {
  width: 100%;
  padding: 12px;
  background: #2D3748;
  color: #F8F9FA;
  border: none;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.15s ease, opacity 0.15s ease;
  margin-top: 8px;

  &:hover:not(:disabled) {
    background: #3B68CC;
  }

  &:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  &.loading {
    background: var(--text-muted);
  }
}

.spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #FFFFFF;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.form-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 20px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;

  .footer-text {
    color: var(--text-muted);
  }

  .footer-link {
    color: #3B68CC;
    cursor: pointer;
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }
}
</style>
