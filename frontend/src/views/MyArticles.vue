<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getArticles, deleteArticle } from '@/api/article'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const articles = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 获取我的文章列表
async function fetchMyArticles() {
  loading.value = true
  try {
    // 获取所有文章，然后过滤出当前用户的
    const res = await getArticles({
      page: 1,
      page_size: 100 // 获取足够多的文章
    })
    const allArticles = res.data || res || []
    // 过滤出当前用户的文章
    articles.value = allArticles.filter(
      article => article.user?.id === authStore.currentUser?.id
    )
    total.value = articles.value.length
  } catch (error) {
    console.error('获取文章失败:', error)
  } finally {
    loading.value = false
  }
}

// 删除文章
async function handleDeleteArticle(articleId) {
  try {
    await ElMessageBox.confirm('确定删除这篇文章吗？删除后无法恢复。', '确认删除', {
      type: 'warning'
    })
    await deleteArticle(articleId)
    ElMessage.success('删除成功')
    // 重新获取列表
    await fetchMyArticles()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除失败:', error)
    }
  }
}

// 编辑文章
function handleEditArticle(articleId) {
  router.push(`/article/edit/${articleId}`)
}

// 查看文章
function handleViewArticle(articleId) {
  router.push(`/article/${articleId}`)
}

// 格式式时间
function formatDate(dateStr) {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  })
}

onMounted(() => {
  fetchMyArticles()
})
</script>

<template>
  <div class="my-articles-page">
    <div class="container">
      <!-- 页面标题 -->
      <div class="page-header">
        <div class="header-left">
          <h1 class="page-title">我的文章</h1>
          <span class="article-count">共 {{ total }} 篇</span>
        </div>
        <button class="btn-write" @click="router.push('/article/edit')">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          写文章
        </button>
      </div>

      <div class="section-divider"></div>

      <!-- 文章列表 -->
      <div v-loading="loading" class="articles-list">
        <template v-if="articles.length > 0">
          <div
            v-for="article in articles"
            :key="article.id"
            class="article-item"
          >
            <div class="article-info" @click="handleViewArticle(article.id)">
              <h3 class="article-title">{{ article.title }}</h3>
              <p class="article-summary">{{ article.summary || '暂无摘要' }}</p>
              <div class="article-meta">
                <span class="meta-date">{{ formatDate(article.created_at) }}</span>
                <span class="meta-dot">·</span>
                <span class="meta-stats">
                  {{ article.view_count || 0 }} 阅读
                  · {{ article.like_count || 0 }} 点赞
                  · {{ article.comment_count || 0 }} 评论
                </span>
              </div>
            </div>
            <div class="article-actions">
              <button
                class="action-btn edit"
                @click="handleEditArticle(article.id)"
                title="编辑"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </button>
              <button
                class="action-btn delete"
                @click="handleDeleteArticle(article.id)"
                title="删除"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                </svg>
              </button>
            </div>
          </div>
        </template>

        <!-- 空状态 -->
        <div v-else-if="!loading" class="empty-state">
          <div class="empty-icon">📝</div>
          <p class="empty-text">你还没有发布过文章</p>
          <button class="btn-primary" @click="router.push('/article/edit')">
            写第一篇文章
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.my-articles-page {
  padding-top: 20px;
  padding-bottom: 40px;
}

// ── 页面标题 ───────────────────────────────────────────
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.header-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.page-title {
  font-family: 'JetBrains Mono', monospace;
  font-size: 20px;
  font-weight: 700;
  color: #2D3748;
  margin: 0;
}

.article-count {
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  color: #718096;
}

.btn-write {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: #5B8DEF;
  color: white;
  border: none;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    background: #4A7DE0;
  }
}

.section-divider {
  height: 2px;
  background: #E2E8F0;
  margin-bottom: 16px;
}

// ── 文章列表 ───────────────────────────────────────────
.articles-list {
  min-height: 400px;
}

.article-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 20px;
  background: white;
  border: 1px solid #E2E8F0;
  border-radius: 8px;
  margin-bottom: 12px;
  transition: all 0.15s ease;

  &:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  }
}

.article-info {
  flex: 1;
  cursor: pointer;
  min-width: 0;
}

.article-title {
  font-size: 17px;
  font-weight: 600;
  color: #2D3748;
  margin: 0 0 8px;
  line-height: 1.4;
  transition: color 0.15s ease;

  .article-item:hover & {
    color: #5B8DEF;
  }
}

.article-summary {
  font-size: 14px;
  color: #718096;
  margin: 0 0 10px;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.article-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  color: #A0AEC0;

  .meta-dot {
    color: #CBD5E0;
  }
}

// ── 操作按钮 ───────────────────────────────────────────
.article-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: #F7FAFC;
  border: 1px solid #E2E8F0;
  border-radius: 6px;
  color: #718096;
  cursor: pointer;
  transition: all 0.15s ease;

  &.edit:hover {
    border-color: #5B8DEF;
    color: #5B8DEF;
    background: #EBF4FF;
  }

  &.delete:hover {
    border-color: #FC8181;
    color: #FC8181;
    background: #FFF5F5;
  }
}

// ── 空状态 ─────────────────────────────────────────────
.empty-state {
  text-align: center;
  padding: 80px 0;

  .empty-icon {
    font-size: 48px;
    margin-bottom: 16px;
  }

  .empty-text {
    font-family: 'JetBrains Mono', monospace;
    font-size: 16px;
    color: #718096;
    margin-bottom: 24px;
  }
}

.btn-primary {
  padding: 10px 24px;
  background: #5B8DEF;
  color: white;
  border: none;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    background: #4A7DE0;
  }
}

// ── 响应式 ─────────────────────────────────────────────
@media (max-width: 640px) {
  .article-item {
    flex-direction: column;
  }

  .article-actions {
    align-self: flex-end;
  }
}
</style>
