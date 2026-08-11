<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox } from 'element-plus'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const searchKeyword = ref('')
const searchFocused = ref(false)
const isScrolled = ref(false)

const isAuthenticated = computed(() => authStore.isAuthenticated)
const currentUser = computed(() => authStore.currentUser)
const isHome = computed(() => route.name === 'Home')

function handleScroll() {
  isScrolled.value = window.scrollY > 18
}

function handleSearch() {
  const keyword = searchKeyword.value.trim()
  if (keyword) {
    router.push({ path: '/search', query: { keyword } })
  }
}

function goToLatest() {
  if (isHome.value) {
    document.querySelector('#latest')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  router.push({ path: '/', hash: '#latest' })
}

async function handleLogout() {
  try {
    await ElMessageBox.confirm('确定要退出当前账号吗？', '退出登录', {
      confirmButtonText: '退出',
      cancelButtonText: '取消',
      type: 'warning'
    })
    authStore.logout()
    router.push('/login')
  } catch {}
}

function handleCommand(command) {
  const destination = {
    'my-articles': '/my-articles',
    favorites: '/favorites',
    profile: '/profile'
  }[command]

  if (destination) router.push(destination)
  if (command === 'logout') handleLogout()
}

onMounted(() => {
  handleScroll()
  window.addEventListener('scroll', handleScroll, { passive: true })
})

onBeforeUnmount(() => window.removeEventListener('scroll', handleScroll))
</script>

<template>
  <div class="layout">
    <header class="nav" :class="{ 'nav--home': isHome, 'is-scrolled': isScrolled }">
      <div class="nav-inner container">
        <button class="nav-logo" aria-label="返回首页" @click="router.push('/')">
          <span class="logo-mark">M</span>
          <span class="logo-copy">
            <strong>墨栈</strong>
          </span>
        </button>

        <nav class="nav-links" aria-label="主导航">
          <button type="button" @click="router.push('/')">发现</button>
          <button type="button" @click="goToLatest">最新文章</button>
        </nav>

        <div class="nav-tools">
          <div class="search-wrapper" :class="{ focused: searchFocused }">
            <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <circle cx="11" cy="11" r="6.5" />
              <path d="m16 16 4.2 4.2" />
            </svg>
            <input
              v-model="searchKeyword"
              type="search"
              aria-label="搜索文章"
              placeholder="搜索灵感、技术与想法"
              @focus="searchFocused = true"
              @blur="searchFocused = false"
              @keyup.enter="handleSearch"
            />
            <button v-if="searchKeyword" class="clear-btn" type="button" aria-label="清空搜索" @click="searchKeyword = ''">×</button>
          </div>

          <template v-if="isAuthenticated">
            <button class="btn-write" type="button" @click="router.push('/article/edit')">
              <span aria-hidden="true">＋</span> 写文章
            </button>
            <el-dropdown trigger="click" @command="handleCommand">
              <button class="nav-user" type="button" :aria-label="`${currentUser?.username || '用户'}的菜单`">
                {{ currentUser?.username?.charAt(0)?.toUpperCase() || '我' }}
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="my-articles"><el-icon><Document /></el-icon>我的文章</el-dropdown-item>
                  <el-dropdown-item command="favorites"><el-icon><Star /></el-icon>我的收藏</el-dropdown-item>
                  <el-dropdown-item command="profile"><el-icon><User /></el-icon>个人中心</el-dropdown-item>
                  <el-dropdown-item divided command="logout"><el-icon><SwitchButton /></el-icon>退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>

          <template v-else>
            <button class="btn-login" type="button" @click="router.push('/login')">登录</button>
            <button class="btn-join" type="button" @click="router.push('/register')">开始写作 <span aria-hidden="true">↗</span></button>
          </template>
        </div>
      </div>
    </header>

    <main class="main" :class="{ 'main--home': isHome }">
      <slot />
    </main>

    <footer class="footer">
      <div class="footer-inner container">
        <div class="footer-brand">
          <span class="footer-mark">M</span>
          <div>
            <strong>墨栈</strong>
            <p>为值得反复阅读的想法留一盏灯。</p>
          </div>
        </div>
        <div class="footer-meta">
          <span>© 2026 墨栈</span>
          <span>Made for slow reading</span>
        </div>
      </div>
    </footer>
  </div>
</template>

<style lang="scss" scoped>
.layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: clip;
}

.nav {
  position: sticky;
  top: 0;
  z-index: 50;
  height: 76px;
  background: rgba(248, 247, 243, 0.94);
  border-bottom: 1px solid rgba(25, 39, 50, 0.1);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  transition: background 0.28s ease, border-color 0.28s ease, color 0.28s ease;
}

.nav--home:not(.is-scrolled) {
  background: linear-gradient(180deg, rgba(4, 18, 31, 0.56), rgba(4, 18, 31, 0));
  border-bottom-color: transparent;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
  color: #f7f3ea;
}

.nav-inner {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 30px;
}

.nav-logo {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: inherit;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  text-align: left;
}

.logo-mark,
.footer-mark {
  display: grid;
  place-items: center;
  width: 31px;
  height: 31px;
  border-radius: 50%;
  background: #d7a462;
  color: #102332;
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 19px;
  font-weight: 700;
  line-height: 1;
}

.logo-copy {
  display: grid;
  gap: 1px;
  line-height: 1;

  strong {
    font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
    font-size: 18px;
    letter-spacing: 0.08em;
  }

  small {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    letter-spacing: 0.18em;
    opacity: 0.65;
  }
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-right: auto;

  button {
    position: relative;
    border: 0;
    background: transparent;
    color: inherit;
    padding: 8px 0;
    font-size: 13px;
    cursor: pointer;
    opacity: 0.72;
    transition: opacity 0.2s ease;

    &::after {
      content: '';
      position: absolute;
      height: 1px;
      left: 0;
      right: 0;
      bottom: 3px;
      background: currentColor;
      transform: scaleX(0);
      transform-origin: left;
      transition: transform 0.22s ease;
    }

    &:hover {
      opacity: 1;
      &::after { transform: scaleX(1); }
    }
  }
}

.nav-tools,
.search-wrapper {
  display: flex;
  align-items: center;
}

.nav-tools {
  gap: 10px;
}

.search-wrapper {
  width: min(24vw, 250px);
  height: 37px;
  padding: 0 10px;
  gap: 8px;
  border: 1px solid rgba(25, 39, 50, 0.14);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.58);
  color: #213b4a;
  transition: width 0.25s ease, background 0.25s ease, border-color 0.25s ease, box-shadow 0.25s ease;

  svg { width: 16px; height: 16px; flex: 0 0 auto; }

  input {
    width: 100%;
    min-width: 0;
    border: 0;
    outline: none;
    background: transparent;
    color: inherit;
    font-size: 12px;

    &::placeholder { color: currentColor; opacity: 0.5; }
  }

  &.focused {
    width: min(29vw, 320px);
    border-color: rgba(176, 117, 50, 0.7);
    background: rgba(255, 255, 255, 0.94);
    box-shadow: 0 0 0 3px rgba(215, 164, 98, 0.14);
  }
}

.nav--home:not(.is-scrolled) .search-wrapper {
  border-color: rgba(255, 255, 255, 0.22);
  background: rgba(1, 16, 30, 0.25);
  color: #f7f3ea;
}

.clear-btn,
.btn-login,
.btn-write,
.btn-join,
.nav-user {
  border: 0;
  cursor: pointer;
  font: inherit;
}

.clear-btn {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: rgba(32, 57, 71, 0.1);
  color: inherit;
  font-size: 17px;
  line-height: 1;
}

.btn-login {
  background: transparent;
  color: inherit;
  padding: 9px 7px;
  font-size: 13px;
}

.btn-join,
.btn-write {
  min-height: 37px;
  padding: 0 14px;
  border-radius: 999px;
  background: #d7a462;
  color: #102332;
  font-size: 12px;
  font-weight: 700;
  transition: transform 0.2s ease, background 0.2s ease;

  &:hover { transform: translateY(-1px); background: #edbd7b; }
}

.btn-join span { margin-left: 4px; font-size: 15px; }
.btn-write span { font-size: 16px; vertical-align: -1px; }

.nav-user {
  width: 37px;
  height: 37px;
  border-radius: 50%;
  background: #244658;
  color: #f8f5ed;
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 16px;
  transition: transform 0.2s ease;
  &:hover { transform: rotate(-7deg); }
}

:deep(.el-dropdown-menu) {
  border: 1px solid rgba(25, 39, 50, 0.11);
  border-radius: 14px;
  padding: 6px;
  box-shadow: 0 16px 36px rgba(13, 29, 40, 0.15);
}

:deep(.el-dropdown-menu__item) {
  border-radius: 8px;
  font-size: 13px;
  padding: 9px 13px;
  .el-icon { margin-right: 7px; }
}

.main {
  flex: 1;
  padding: 46px 0 70px;
}

.main--home {
  margin-top: -76px;
  padding: 0;
}

.footer {
  margin-top: auto;
  padding: 34px 0;
  background: #102332;
  color: rgba(249, 246, 238, 0.82);
}

.footer-inner,
.footer-brand,
.footer-meta {
  display: flex;
  align-items: center;
}

.footer-inner { justify-content: space-between; gap: 20px; }
.footer-brand { gap: 11px; }
.footer-mark { width: 29px; height: 29px; font-size: 17px; }

.footer-brand strong {
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: 16px;
  letter-spacing: 0.08em;
}

.footer-brand p,
.footer-meta {
  margin: 2px 0 0;
  font-size: 11px;
  color: rgba(249, 246, 238, 0.52);
}

.footer-meta { gap: 18px; font-family: 'JetBrains Mono', monospace; font-size: 11px; letter-spacing: 0.08em; }

@media (max-width: 820px) {
  .nav { height: 68px; }
  .nav-inner { gap: 14px; }
  .nav-links { display: none; }
  .nav-tools { margin-left: auto; }
  .search-wrapper { width: 39px; padding: 0; justify-content: center; background: transparent; border-color: transparent; }
  .search-wrapper input, .search-wrapper .clear-btn { display: none; }
  .search-wrapper.focused { width: min(45vw, 260px); padding: 0 10px; justify-content: flex-start; }
  .search-wrapper.focused input, .search-wrapper.focused .clear-btn { display: block; }
  .btn-login { display: none; }
  .main--home { margin-top: -68px; }
}

@media (max-width: 540px) {
  .container { padding-left: 18px; padding-right: 18px; }
  .logo-copy small { display: none; }
  .logo-copy strong { font-size: 17px; }
  .btn-join, .btn-write { padding: 0 11px; }
  .btn-join { font-size: 0; width: 37px; padding: 0; }
  .btn-join span { margin: 0; font-size: 16px; }
  .footer-inner { align-items: flex-start; flex-direction: column; }
  .footer-meta { gap: 10px; flex-wrap: wrap; }
}
</style>
