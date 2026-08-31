<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'

const props = defineProps({
  article: { type: Object, required: true },
  index: { type: Number, default: 0 }
})

const router = useRouter()

const excerpt = computed(() => {
  const source = props.article.summary || props.article.content || ''
  return source.replace(/[#>*_`\[\]()]/g, ' ').replace(/\s+/g, ' ').trim()
})

function goToDetail() {
  router.push(`/article/${props.article.id}`)
}

function formatTime(dateStr) {
  if (!dateStr) return '刚刚发布'
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now - date
  const days = Math.floor(diff / 86400000)

  if (days < 1) return '今天'
  if (days < 7) return `${days} 天前`
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

function estimateReadTime(content) {
  return Math.max(1, Math.ceil((content || '').length / 500))
}
</script>

<template>
  <article class="article-card" tabindex="0" role="link" @click="goToDetail" @keyup.enter="goToDetail">
    <span class="card-index">{{ String(index + 1).padStart(2, '0') }}</span>

    <div class="card-body">
      <div class="card-topline">
        <span>{{ formatTime(article.created_at) }}</span>
        <span v-if="article.tags?.[0]">{{ article.tags[0].name }}</span>
      </div>
      <h3>{{ article.title }}</h3>
      <p v-if="excerpt" class="card-summary">{{ excerpt }}</p>
      <div class="card-bottom">
        <span>作者：{{ article.user?.username || '随想录' }}</span>
        <span>阅读 {{ estimateReadTime(article.content) }} 分钟</span>
        <span class="card-reactions"><i aria-hidden="true">♡</i>{{ article.like_count || 0 }} <i aria-hidden="true">◌</i>{{ article.comment_count || 0 }}</span>
      </div>
    </div>

    <div v-if="article.cover_image" class="card-cover">
      <img :src="article.cover_image" :alt="article.title" />
    </div>
    <span class="card-arrow" aria-hidden="true">↗</span>
  </article>
</template>

<style lang="scss" scoped>
.article-card {
  position: relative;
  display: flex;
  gap: 22px;
  align-items: flex-start;
  padding: 29px 48px 29px 4px;
  border-bottom: 1px solid rgba(25, 54, 66, 0.15);
  color: #183340;
  cursor: pointer;
  outline: none;
  transition: padding-left .25s ease, background .25s ease;

  &:hover { padding-left: 13px; background: rgba(235, 229, 217, .38); }
  &:focus-visible { box-shadow: inset 3px 0 0 #c28b4d; }
  &:hover .card-arrow { opacity: 1; transform: translate(0, -2px); }
  &:hover h3 { color: #ab6f33; }
  &:hover .card-cover img { transform: scale(1.05); }
}

.card-index {
  flex: 0 0 34px;
  padding-top: 5px;
  color: #92571e;
  font: 12px 'JetBrains Mono', 'SFMono-Regular', monospace;
  letter-spacing: .08em;
}

.card-body { flex: 1; min-width: 0; }
.card-topline { display: flex; gap: 12px; color: #52666f; font: 12px 'JetBrains Mono', 'SFMono-Regular', monospace; letter-spacing: .06em; }.card-topline span + span { display: flex; align-items: center; gap: 12px; color: #92571e; }.card-topline span + span::before { content: ''; width: 14px; height: 1px; background: currentColor; opacity: .75; }

h3 {
  margin: 10px 0 0;
  color: #1a3441;
  font-family: 'Noto Serif SC', 'Songti SC', STSong, SimSun, serif;
  font-size: 22px;
  font-weight: 600;
  line-height: 1.45;
  letter-spacing: .035em;
  transition: color .22s ease;
}

.card-summary {
  display: -webkit-box;
  max-width: 700px;
  margin: 8px 0 0;
  overflow: hidden;
  color: #566a72;
  font-size: 14px;
  line-height: 1.8;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.card-bottom { display: flex; flex-wrap: wrap; gap: 8px 16px; margin-top: 15px; color: #586c74; font: 12px 'JetBrains Mono', 'SFMono-Regular', monospace; letter-spacing: .04em; }.card-reactions { margin-left: auto; letter-spacing: 0; }.card-reactions i { margin-right: 4px; font-size: 14px; font-style: normal; vertical-align: -1px; }.card-reactions i + i { margin-left: 10px; }

.card-cover { flex: 0 0 132px; height: 92px; margin-top: 1px; overflow: hidden; }.card-cover img { width: 100%; height: 100%; object-fit: cover; transition: transform .5s ease; }
.card-arrow { position: absolute; right: 11px; top: 50%; color: #ac7137; font-size: 21px; opacity: 0; transform: translate(-8px, 4px); transition: opacity .22s ease, transform .22s ease; }

@media (max-width: 640px) {
  .article-card { gap: 12px; padding: 24px 25px 24px 0; }.article-card:hover { padding-left: 5px; }.card-index { flex-basis: 24px; font-size: 12px; }.card-cover { display: none; }h3 { font-size: 20px; }.card-summary { font-size: 14px; }.card-bottom { margin-top: 12px; }.card-reactions { width: 100%; margin-left: 0; }.card-arrow { right: 4px; opacity: 1; transform: translateY(-50%); }
}
</style>
