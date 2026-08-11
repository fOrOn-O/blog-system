<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  getArticle, deleteArticle,
  likeArticle, unlikeArticle, getLikeInfo,
  favoriteArticle, unfavoriteArticle, checkFavorited,
  getComments, createComment, deleteComment
} from '@/api/article'
import { ElMessage, ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const article = ref(null)
const comments = ref([])
const loading = ref(false)
const commentText = ref('')
const submittingComment = ref(false)
const likeInfo = ref({ count: 0, is_liked: false })
const isFavorited = ref(false)

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAuthor = computed(() => authStore.currentUser?.id === article.value?.user?.id)
const articleId = computed(() => route.params.id)

function formatDate(dateStr) {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleDateString('zh-CN', {
    year: 'numeric', month: 'long', day: 'numeric'
  })
}

function estimateReadTime(content) {
  if (!content) return '1'
  return Math.max(1, Math.ceil(content.length / 500))
}

const messages = {
  notFound: '文章不存在',
  deleted: '已删除',
  addedFav: '已收藏',
  removedFav: '已取消收藏',
  commentEmpty: '请输入评论内容',
  commentSuccess: '评论成功',
  deleteComment: '确定删除这条评论吗？',
  deleteArticle: '确定删除这篇文章吗？删除后无法恢复。',
  loginFirst: '请先登录',
}

async function fetchArticle() {
  loading.value = true
  try {
    const res = await getArticle(articleId.value)
    article.value = res.data || res
    await Promise.all([
      fetchLikeInfo(),
      fetchComments(),
      isAuthenticated.value ? fetchFavoriteStatus() : Promise.resolve()
    ])
  } catch (error) {
    ElMessage.error(messages.notFound)
    router.push('/')
  } finally {
    loading.value = false
  }
}

async function fetchLikeInfo() {
  try {
    const res = await getLikeInfo(articleId.value)
    const data = res.data || res
    likeInfo.value = { count: data.count || data.like_count || 0, is_liked: data.is_liked || false }
  } catch {}
}

async function fetchFavoriteStatus() {
  try {
    const res = await checkFavorited(articleId.value)
    isFavorited.value = (res.data || res).is_favorited || false
  } catch {}
}

async function fetchComments() {
  try {
    const res = await getComments(articleId.value)
    comments.value = res.data || res || []
  } catch {}
}

async function handleLike() {
  if (!isAuthenticated.value) { router.push('/login'); return }
  try {
    if (likeInfo.value.is_liked) {
      await unlikeArticle(articleId.value)
      likeInfo.value.count--
      likeInfo.value.is_liked = false
    } else {
      await likeArticle(articleId.value)
      likeInfo.value.count++
      likeInfo.value.is_liked = true
    }
  } catch {}
}

async function handleFavorite() {
  if (!isAuthenticated.value) { router.push('/login'); return }
  try {
    if (isFavorited.value) {
      await unfavoriteArticle(articleId.value)
      isFavorited.value = false
      ElMessage.success(messages.removedFav)
    } else {
      await favoriteArticle(articleId.value)
      isFavorited.value = true
      ElMessage.success(messages.addedFav)
    }
  } catch {}
}

async function handleSubmitComment() {
  if (!isAuthenticated.value) { router.push('/login'); return }
  if (!commentText.value.trim()) { ElMessage.warning(messages.commentEmpty); return }
  submittingComment.value = true
  try {
    await createComment(articleId.value, { content: commentText.value.trim() })
    ElMessage.success(messages.commentSuccess)
    commentText.value = ''
    await fetchComments()
  } catch {} finally {
    submittingComment.value = false
  }
}

async function handleDeleteComment(commentId) {
  try {
    await ElMessageBox.confirm(messages.deleteComment, '确认删除', { type: 'warning' })
    await deleteComment(commentId)
    ElMessage.success(messages.deleted)
    await fetchComments()
  } catch {}
}

async function handleDeleteArticle() {
  try {
    await ElMessageBox.confirm(messages.deleteArticle, '确认删除', { type: 'warning' })
    await deleteArticle(articleId.value)
    ElMessage.success(messages.deleted)
    router.push('/')
  } catch {}
}

onMounted(() => { fetchArticle() })
</script>

<template>
  <div class="detail-page" v-loading="loading">
    <div class="container" v-if="article">
      <!-- Back -->
      <div class="back-link" @click="router.push('/')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/>
        </svg>
        <span>返回</span>
      </div>

      <!-- Article -->
      <article class="article">
        <!-- Header -->
        <header class="article-header">
          <div class="article-meta-top">
            <span class="meta-author">@{{ article.user?.username || '匿名' }}</span>
            <span class="meta-dot">·</span>
            <span class="meta-date">{{ formatDate(article.created_at) }}</span>
            <span class="meta-dot">·</span>
            <span class="meta-read">约{{ estimateReadTime(article.content) }}分钟</span>
          </div>

          <h1 class="article-title">{{ article.title }}</h1>

          <div v-if="isAuthor" class="article-actions">
            <button class="action-btn" @click="router.push(`/article/edit/${articleId}`)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
              </svg>
              编辑
            </button>
            <button class="action-btn danger" @click="handleDeleteArticle">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
              </svg>
              删除
            </button>
          </div>
        </header>

        <!-- Divider -->
        <div class="article-divider"></div>

        <!-- Content -->
        <div class="article-content prose" v-html="article.content"></div>

        <!-- Interaction Bar -->
        <div class="interaction-bar">
          <button
            class="inter-btn"
            :class="{ active: likeInfo.is_liked }"
            @click="handleLike"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" :fill="likeInfo.is_liked ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2">
              <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/>
            </svg>
            <span>{{ likeInfo.count }}</span>
          </button>

          <button
            class="inter-btn"
            :class="{ active: isFavorited }"
            @click="handleFavorite"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" :fill="isFavorited ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2">
              <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
            </svg>
            <span>{{ isFavorited ? '已收藏' : '收藏' }}</span>
          </button>

          <button class="inter-btn">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
            </svg>
            <span>{{ comments.length }}</span>
          </button>
        </div>
      </article>

      <!-- Comments Section -->
      <section class="comments-section">
        <div class="comments-header">
          <span class="comments-label">评论</span>
          <span class="comments-count">{{ comments.length }}</span>
        </div>
        <div class="section-divider"></div>

        <!-- Comment Input -->
        <div class="comment-input-box">
          <template v-if="isAuthenticated">
            <div class="input-row">
              <div class="input-avatar">
                {{ authStore.currentUser?.username?.charAt(0)?.toUpperCase() }}
              </div>
              <div class="input-body">
                <textarea
                  v-model="commentText"
                  class="comment-textarea"
                  placeholder="写下你的评论..."
                  rows="3"
                  maxlength="500"
                ></textarea>
                <div class="input-footer">
                  <span class="char-count">{{ commentText.length }}/500</span>
                  <button
                    class="submit-btn"
                    :disabled="submittingComment || !commentText.trim()"
                    @click="handleSubmitComment"
                  >
                    {{ submittingComment ? '发布中...' : '→ 发布' }}
                  </button>
                </div>
              </div>
            </div>
          </template>
          <div v-else class="login-prompt">
            <span>登录后参与讨论</span>
            <button class="prompt-btn" @click="router.push('/login')">登录</button>
          </div>
        </div>

        <!-- Comment List -->
        <div class="comment-list">
          <div
            v-for="comment in comments"
            :key="comment?.id"
            class="comment-item"
          >
            <div class="comment-avatar">
              {{ comment?.user?.username?.charAt(0)?.toUpperCase() || '?' }}
            </div>
            <div class="comment-body">
              <div class="comment-meta">
                <span class="comment-author">{{ comment?.user?.username || '匿名' }}</span>
                <span class="comment-time">{{ formatDate(comment?.created_at) }}</span>
              </div>
              <p class="comment-text">{{ comment?.content }}</p>
              <button
                v-if="comment?.user_id && authStore.currentUser?.id === comment.user_id"
                class="comment-delete"
                @click="handleDeleteComment(comment.id)"
              >
                删除
              </button>
            </div>
          </div>

          <div v-if="comments.length === 0" class="empty-comments">
            <span class="empty-icon">//</span>
            <span>暂无评论，快来抢沙发吧！</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.detail-page {
  padding-top: 8px;
  padding-bottom: 64px;
}

.container {
  max-width: 720px;
}

// ── Back Link ──────────────────────────────────────────
.back-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  color: #8896AB;
  cursor: pointer;
  margin-bottom: 24px;
  transition: color 0.15s ease;

  &:hover {
    color: #5B8DEF;
  }
}

// ── Article ────────────────────────────────────────────
.article {
  background: #FFFFFF;
  border: 1px solid #E8ECF0;
  border-radius: 8px;
  padding: 40px;
}

.article-header {
  .article-meta-top {
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: #8896AB;
    margin-bottom: 16px;

    .meta-dot {
      color: #E8ECF0;
    }
  }

  .article-title {
    font-family: 'Inter', sans-serif;
    font-size: 32px;
    font-weight: 700;
    color: #2D3748;
    line-height: 1.3;
    margin: 0 0 16px;
    letter-spacing: -0.02em;
  }

  .article-actions {
    display: flex;
    gap: 8px;
  }
}

.action-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: #F8F9FA;
  border: 1px solid #E8ECF0;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  color: #8896AB;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    border-color: #2D3748;
    color: #2D3748;
  }

  &.danger:hover {
    border-color: #FC8181;
    color: #FC8181;
    background: #FFF5F5;
  }
}

.article-divider {
  height: 1px;
  background: #E8ECF0;
  margin: 24px 0;
}

.article-content {
  :deep(h2) {
    font-family: 'Inter', sans-serif;
    font-size: 24px;
    font-weight: 600;
    margin-top: 40px;
    margin-bottom: 16px;
    padding-bottom: 8px;
    border-bottom: 1px solid #E8ECF0;
  }

  :deep(h3) {
    font-size: 20px;
    font-weight: 600;
  }

  :deep(p) {
    margin-bottom: 16px;
    line-height: 1.8;
  }

  :deep(img) {
    border-radius: 6px;
    border: 1px solid #E8ECF0;
    margin: 24px 0;
  }

  :deep(a) {
    color: #5B8DEF;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  :deep(code) {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.88em;
    background: #EDF2F7;
    padding: 2px 6px;
    border-radius: 4px;
    color: #5B8DEF;
  }

  :deep(pre) {
    background: #2D3748;
    color: #E2E8F0;
    padding: 20px;
    border-radius: 6px;
    overflow-x: auto;
    margin: 24px 0;
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    line-height: 1.7;

    code {
      background: none;
      padding: 0;
      color: inherit;
    }
  }

  :deep(blockquote) {
    border-left: 3px solid #5B8DEF;
    padding: 12px 20px;
    margin: 24px 0;
    background: #F8F9FA;
    border-radius: 0 6px 6px 0;
    color: #8896AB;
    font-style: italic;
  }
}

// ── Interaction Bar ────────────────────────────────────
.interaction-bar {
  display: flex;
  gap: 8px;
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid #E8ECF0;
}

.inter-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: #F8F9FA;
  border: 1px solid #E8ECF0;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  color: #8896AB;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    border-color: #5B8DEF;
    color: #5B8DEF;
  }

  &.active {
    border-color: #5B8DEF;
    color: #5B8DEF;
    background: #EBF2FF;
  }
}

// ── Comments ───────────────────────────────────────────
.comments-section {
  margin-top: 24px;
  background: #FFFFFF;
  border: 1px solid #E8ECF0;
  border-radius: 8px;
  padding: 28px 40px;
}

.comments-header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 8px;

  .comments-label {
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #2D3748;
  }

  .comments-count {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    color: #8896AB;
  }
}

.comment-input-box {
  margin: 20px 0;
}

.input-row {
  display: flex;
  gap: 12px;
}

.input-avatar {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  background: #5B8DEF;
  color: #FFFFFF;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'JetBrains Mono', monospace;
  font-size: 14px;
  font-weight: 600;
  flex-shrink: 0;
}

.input-body {
  flex: 1;
}

.comment-textarea {
  width: 100%;
  padding: 12px;
  background: #F8F9FA;
  border: 1px solid #E8ECF0;
  border-radius: 6px;
  font-family: 'Inter', sans-serif;
  font-size: 14px;
  color: #2D3748;
  resize: none;
  outline: none;
  transition: border-color 0.15s ease;

  &::placeholder {
    color: #A0AEC0;
  }

  &:focus {
    border-color: #5B8DEF;
  }
}

.input-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;

  .char-count {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: #A0AEC0;
  }
}

.submit-btn {
  padding: 6px 16px;
  background: #2D3748;
  color: #F8F9FA;
  border: none;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover:not(:disabled) {
    background: #5B8DEF;
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
}

.login-prompt {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px;
  background: #F8F9FA;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  color: #8896AB;

  .prompt-btn {
    padding: 4px 12px;
    background: #5B8DEF;
    color: #FFFFFF;
    border: none;
    border-radius: 4px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    cursor: pointer;
    transition: background 0.15s ease;

    &:hover {
      background: #4A7DE0;
    }
  }
}

.comment-list {
  margin-top: 16px;
}

.comment-item {
  display: flex;
  gap: 12px;
  padding: 16px 0;
  border-bottom: 1px solid #EDF2F7;

  &:last-child {
    border-bottom: none;
  }
}

.comment-avatar {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  background: #E8ECF0;
  color: #8896AB;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}

.comment-body {
  flex: 1;
  min-width: 0;
}

.comment-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;

  .comment-author {
    font-family: 'JetBrains Mono', monospace;
    font-size: 13px;
    font-weight: 600;
    color: #2D3748;
  }

  .comment-time {
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    color: #A0AEC0;
  }
}

.comment-text {
  font-size: 14px;
  color: #2D3748;
  line-height: 1.6;
  margin: 0;
}

.comment-delete {
  margin-top: 6px;
  background: none;
  border: none;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: #A0AEC0;
  cursor: pointer;
  transition: color 0.15s ease;
  padding: 0;

  &:hover {
    color: #FC8181;
  }
}

.empty-comments {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 32px 0;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  color: #A0AEC0;
  justify-content: center;

  .empty-icon {
    color: #E8ECF0;
    font-weight: 700;
  }
}

// ── Responsive ─────────────────────────────────────────
@media (max-width: 768px) {
  .article {
    padding: 24px;
  }

  .article-header .article-title {
    font-size: 24px;
  }

  .comments-section {
    padding: 20px 24px;
  }
}
</style>
