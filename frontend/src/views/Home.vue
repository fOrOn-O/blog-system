<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getArticles } from '@/api/article'
import ArticleCard from '@/components/ArticleCard.vue'
import heroImage from '@/assets/editorial-hero.png'

const router = useRouter()
const route = useRoute()
const articles = ref([])
const loading = ref(false)
const initialPage = Number.parseInt(route.query.page, 10)
const currentPage = ref(Number.isInteger(initialPage) && initialPage > 0 ? initialPage : 1)
const pageSize = ref(10)
const total = ref(0)

const stats = ref({ articles: 0, likes: 0, comments: 0 })

const topics = [
  { en: 'BUILD', cn: '工程与架构', query: 'Go', note: '把复杂问题拆成清晰路径' },
  { en: 'CRAFT', cn: '前端与体验', query: 'Vue', note: '让每一次交互都恰如其分' },
  { en: 'THINK', cn: '方法与洞见', query: '效率', note: '记录那些经过验证的想法' }
]

const tags = ['Go', 'Vue', 'JavaScript', 'Docker', '数据库', 'AI', '工程化', '设计']

const featuredArticle = ref(null)
const allArticles = computed(() => articles.value)
const featureCover = computed(() => featuredArticle.value?.cover_image || heroImage)
const displayTotal = computed(() => String(total.value || 0).padStart(2, '0'))

function extractArticles(response) {
  return Array.isArray(response?.data) ? response.data : Array.isArray(response) ? response : []
}

async function fetchArticles() {
  loading.value = true
  try {
    const res = await getArticles({ page: currentPage.value, limit: pageSize.value })
    articles.value = extractArticles(res)

    if (currentPage.value === 1) {
      featuredArticle.value = articles.value[0] || null
    } else if (!featuredArticle.value) {
      const latestRes = await getArticles({ page: 1, limit: 1 })
      featuredArticle.value = extractArticles(latestRes)[0] || null
    }
    total.value = res?.total || res?.meta?.total || articles.value.length
    stats.value.articles = total.value
    stats.value.likes = articles.value.reduce((sum, article) => sum + (article.like_count || 0), 0)
    stats.value.comments = articles.value.reduce((sum, article) => sum + (article.comment_count || 0), 0)
  } catch (error) {
    console.error('Failed to fetch articles:', error)
  } finally {
    loading.value = false
  }
}

function getExcerpt(article) {
  const source = article?.summary || article?.content || ''
  const plainText = source.replace(/[#>*_`\[\]()]/g, ' ').replace(/\s+/g, ' ').trim()
  return plainText || '一篇等待被慢慢读完的文章。'
}

function formatDate(dateString) {
  if (!dateString) return '刚刚发布'
  return new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric' }).format(new Date(dateString))
}

function estimateReadTime(article) {
  return Math.max(1, Math.ceil((article?.content || '').length / 500))
}

function goToArticle(id) {
  router.push(`/article/${id}`)
}

function scrollToLatest() {
  document.querySelector('#featured')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function searchTopic(query) {
  router.push({ path: '/search', query: { keyword: query } })
}

async function handlePageChange(page) {
  currentPage.value = page
  await router.replace({
    name: 'Home',
    query: {
      ...route.query,
      page: page > 1 ? String(page) : undefined
    }
  })
  await fetchArticles()
  document.querySelector('#latest-notes')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

onMounted(fetchArticles)
</script>

<template>
  <div class="home">
    <section class="hero" :style="{ '--hero-image': `url(${heroImage})` }">
      <div class="hero-noise" aria-hidden="true"></div>
      <div class="hero-content container">
        <p class="hero-kicker reveal reveal--one"><span></span> 随想录 · 2026</p>
        <h1 class="reveal reveal--two">把每一次思考，<br />写成值得重读的篇章。</h1>
        <p class="hero-copy reveal reveal--three">这里收录技术、创造与日常的深度记录。<br />慢一点读，也认真一点写。</p>
        <div class="hero-actions reveal reveal--four">
          <button class="hero-primary" type="button" @click="scrollToLatest">开始阅读 <span aria-hidden="true">↓</span></button>
          <button class="hero-secondary" type="button" @click="router.push('/article/edit')">写下此刻 <span aria-hidden="true">↗</span></button>
        </div>
      </div>
      <div class="hero-footer container reveal reveal--four">
        <div class="hero-issue"><span>ISSUE</span><strong>01</strong><i></i><em>夏日的技术笔记</em></div>
        <button class="scroll-cue" type="button" aria-label="滚动到最新文章" @click="scrollToLatest"><span>SCROLL TO EXPLORE</span><i></i></button>
      </div>
    </section>

    <section id="featured" class="latest container">
      <div class="section-intro">
        <div>
          <p class="eyebrow">THE READING ROOM</p>
          <h2>本周新知</h2>
        </div>
        <p>来自每一个认真记录的人。<br />共 <strong>{{ displayTotal }}</strong> 篇可供阅读。</p>
      </div>

      <div v-loading="loading" class="reading-space">
        <template v-if="featuredArticle">
          <article class="feature-story" tabindex="0" role="link" @click="goToArticle(featuredArticle.id)" @keyup.enter="goToArticle(featuredArticle.id)">
            <div class="feature-image">
              <img :src="featureCover" :alt="featuredArticle.title" />
              <span class="image-label">FEATURED NOTE</span>
            </div>
            <div class="feature-copy">
              <p class="story-type">编辑精选 <span></span> {{ formatDate(featuredArticle.created_at) }}</p>
              <h3>{{ featuredArticle.title }}</h3>
              <p class="story-summary">{{ getExcerpt(featuredArticle) }}</p>
              <div class="feature-bottom">
                <span>作者：{{ featuredArticle.user?.username || '随想录' }}</span>
                <span>阅读 {{ estimateReadTime(featuredArticle) }} 分钟</span>
                <b>阅读文章 <i aria-hidden="true">↗</i></b>
              </div>
            </div>
          </article>

          <div id="latest-notes" class="journal-heading">
            <p><span></span> LATEST NOTES</p>
            <span>{{ String(currentPage).padStart(2, '0') }} / {{ String(Math.max(1, Math.ceil(total / pageSize))).padStart(2, '0') }}</span>
          </div>

          <div v-if="allArticles.length" class="journal-list">
            <ArticleCard
              v-for="(article, index) in allArticles"
              :key="article.id"
              :article="article"
              :index="(currentPage - 1) * pageSize + index"
              class="animate-in"
              :style="{ animationDelay: `${index * 0.06}s` }"
            />
          </div>

          <div v-else class="single-note">
            <span>01</span>
            <p>新的篇章正在整理中，请先从这篇开始。</p>
          </div>

          <div v-if="total > pageSize" class="pagination">
            <button :disabled="currentPage <= 1" type="button" @click="handlePageChange(currentPage - 1)">← 上一页</button>
            <span>{{ currentPage }} / {{ Math.ceil(total / pageSize) }}</span>
            <button :disabled="currentPage >= Math.ceil(total / pageSize)" type="button" @click="handlePageChange(currentPage + 1)">下一页 →</button>
          </div>
        </template>

        <div v-else-if="!loading" class="first-note">
          <div class="first-note-seal" aria-hidden="true"></div>
          <p class="eyebrow">THE FIRST PAGE IS WAITING</p>
          <h3>从第一篇公开笔记开始，<br />让这里慢慢成为你的知识花园。</h3>
          <p>无需等到一切完美。一个正在成形的想法，就值得被好好记下来。</p>
          <button type="button" @click="router.push('/article/edit')">写第一篇文章 <span aria-hidden="true">↗</span></button>
        </div>
      </div>
    </section>

    <section class="explore">
      <div class="container">
        <div class="explore-top">
          <p class="eyebrow">FOLLOW YOUR CURIOSITY</p>
          <h2>从感兴趣的方向开始</h2>
          <span>03</span>
        </div>
        <div class="topic-grid">
          <button v-for="(topic, index) in topics" :key="topic.en" type="button" class="topic" @click="searchTopic(topic.query)">
            <span class="topic-index">0{{ index + 1 }}</span>
            <span class="topic-en">{{ topic.en }}</span>
            <strong>{{ topic.cn }}</strong>
            <small>{{ topic.note }}</small>
            <i aria-hidden="true">↗</i>
          </button>
        </div>
      </div>
    </section>

    <section class="community container">
      <div class="community-copy">
        <p class="eyebrow">IN THE MARGIN</p>
        <h2>留下你的<br /><em>阅读轨迹。</em></h2>
        <p>收藏触动你的段落，写下正在验证的判断，也让偶遇的灵感在这里继续生长。</p>
        <button type="button" @click="router.push('/register')">加入随想录 <span aria-hidden="true">↗</span></button>
      </div>
      <div class="community-side">
        <div class="side-heading"><span>READING PULSE</span><i></i><b>LIVE</b></div>
        <div class="pulse-stats">
          <div><strong>{{ stats.articles }}</strong><span>篇文章</span></div>
          <div><strong>{{ stats.likes }}</strong><span>次共鸣</span></div>
          <div><strong>{{ stats.comments }}</strong><span>条讨论</span></div>
        </div>
        <div class="tags-wrap">
          <p>正在被阅读</p>
          <div>
            <button v-for="tag in tags" :key="tag" type="button" @click="searchTopic(tag)"># {{ tag }}</button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.home { background: #f8f7f3; color: #18303d; }

.hero {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  min-height: 100svh;
  color: #f8f4ea;
  background-color: #071b2a;
  background-image: linear-gradient(90deg, rgba(3, 16, 28, 0.44) 0%, rgba(3, 16, 28, 0.08) 54%, rgba(3, 16, 28, 0.2) 100%), var(--hero-image);
  background-position: center;
  background-size: cover;
}

.hero::after {
  content: '';
  position: absolute;
  z-index: -1;
  inset: 0;
  background: linear-gradient(180deg, transparent 60%, rgba(2, 14, 24, 0.44));
  pointer-events: none;
}

.hero-noise {
  position: absolute;
  z-index: -1;
  inset: 0;
  opacity: 0.15;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.95' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.45'/%3E%3C/svg%3E");
  pointer-events: none;
}

.hero-content {
  width: 100%;
  margin: auto auto 0;
  padding-top: 180px;
  padding-bottom: 92px;
}

.hero-kicker,
.eyebrow,
.story-type,
.journal-heading,
.side-heading,
.hero-issue,
.scroll-cue {
  font-family: 'JetBrains Mono', 'SFMono-Regular', Consolas, monospace;
  text-transform: uppercase;
  letter-spacing: 0.14em;
}

.hero-kicker {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 0 0 20px;
  color: #e6c189;
  font-size: 12px;
  font-weight: 600;
}

.hero-kicker span { width: 25px; height: 1px; background: currentColor; }

.hero h1,
.section-intro h2,
.explore h2,
.community h2,
.first-note h3 {
  font-family: 'Noto Serif SC', 'Songti SC', STSong, SimSun, serif;
  font-weight: 600;
}

.hero h1 {
  max-width: 760px;
  margin: 0;
  font-size: clamp(46px, 5.25vw, 82px);
  line-height: 1.17;
  letter-spacing: 0.025em;
  text-wrap: balance;
}

.hero-copy {
  margin: 25px 0 0;
  color: var(--text-on-dark-muted);
  font-size: 16px;
  line-height: 1.9;
}

.hero-actions { display: flex; gap: 13px; margin-top: 33px; }

.hero-actions button,
.first-note button,
.community-copy button {
  min-height: 47px;
  border: 0;
  border-radius: 999px;
  padding: 0 22px;
  cursor: pointer;
  font-size: 13px;
  transition: transform 0.22s ease, background 0.22s ease, color 0.22s ease;
}

.hero-primary { background: #d7a462; color: #122a38; font-weight: 700; }
.hero-primary:hover, .first-note button:hover { transform: translateY(-2px); background: #edc184; }
.hero-primary span, .hero-secondary span, .first-note button span, .community-copy button span { margin-left: 9px; font-size: 16px; }

.hero-secondary {
  background: rgba(250, 247, 240, 0.05);
  box-shadow: inset 0 0 0 1px rgba(250, 247, 240, 0.36);
  color: #f8f4ea;
}

.hero-secondary:hover { transform: translateY(-2px); background: rgba(250, 247, 240, 0.14); }

.hero-footer {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  width: 100%;
  padding-bottom: 33px;
}

.hero-issue { display: flex; align-items: center; gap: 9px; color: var(--text-on-dark-muted); font-size: 12px; }
.hero-issue strong { color: #e6c189; font-size: 14px; }
.hero-issue i { width: 38px; height: 1px; background: rgba(248, 244, 234, 0.3); }
.hero-issue em { font-family: inherit; font-style: normal; letter-spacing: 0.08em; }

.scroll-cue {
  display: flex;
  align-items: center;
  gap: 11px;
  border: 0;
  background: transparent;
  color: var(--text-on-dark-muted);
  cursor: pointer;
  font-size: 12px;
}

.scroll-cue i { display: block; width: 37px; height: 1px; background: currentColor; transform-origin: right; transition: transform .25s ease; }
.scroll-cue:hover i { transform: scaleX(1.65); }

.reveal { opacity: 0; animation: rise 0.8s cubic-bezier(.2,.8,.2,1) forwards; }
.reveal--one { animation-delay: 0.12s; }.reveal--two { animation-delay: 0.22s; }.reveal--three { animation-delay: 0.36s; }.reveal--four { animation-delay: 0.48s; }
@keyframes rise { from { opacity: 0; transform: translateY(18px); } to { opacity: 1; transform: translateY(0); } }

.latest { padding-top: 130px; padding-bottom: 126px; scroll-margin-top: 84px; }

.section-intro, .explore-top { display: flex; align-items: flex-end; justify-content: space-between; gap: 30px; }
.eyebrow { margin: 0 0 12px; color: #92571e; font-size: 13px; font-weight: 700; }
.section-intro h2, .explore h2 { margin: 0; color: #173241; font-size: clamp(31px, 3.3vw, 47px); line-height: 1.2; letter-spacing: .04em; }
.section-intro > p { margin: 0 0 5px; color: #52666f; font-size: 14px; line-height: 1.75; text-align: right; }
.section-intro strong { color: #173241; font-family: 'JetBrains Mono', monospace; font-size: 17px; }

.reading-space { margin-top: 53px; min-height: 370px; }

.feature-story {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(340px, .95fr);
  min-height: 432px;
  background: #14303e;
  color: #f8f3e8;
  cursor: pointer;
  outline: none;
}

.feature-story:focus-visible { box-shadow: 0 0 0 4px #d7a462; }
.feature-image { position: relative; overflow: hidden; }
.feature-image::after { content: ''; position: absolute; inset: 0; background: linear-gradient(120deg, rgba(6,25,36,.28), transparent 60%); }
.feature-image img { width: 100%; height: 100%; object-fit: cover; transition: transform .7s cubic-bezier(.2,.7,.2,1); }
.feature-story:hover .feature-image img { transform: scale(1.045); }
.image-label { position: absolute; z-index: 1; top: 21px; left: 21px; padding: 7px 9px; background: rgba(10, 30, 41, .78); color: #f2d9b0; font: 12px 'JetBrains Mono', monospace; letter-spacing: .11em; }

.feature-copy { display: flex; flex-direction: column; padding: clamp(30px, 4.2vw, 58px); }
.story-type { display: flex; align-items: center; gap: 9px; margin: 0; color: #e9c98f; font-size: 12px; }
.story-type span { display: inline-block; width: 18px; height: 1px; background: currentColor; opacity: .6; }
.feature-copy h3 { margin: 21px 0 16px; font-family: 'Noto Serif SC', 'Songti SC', STSong, SimSun, serif; font-size: clamp(25px, 2.4vw, 37px); line-height: 1.37; font-weight: 600; letter-spacing: .03em; }
.story-summary { display: -webkit-box; margin: 0; overflow: hidden; color: rgba(247,242,231,.8); font-size: 15px; line-height: 1.9; -webkit-line-clamp: 3; -webkit-box-orient: vertical; }
.feature-bottom { display: flex; align-items: center; flex-wrap: wrap; gap: 9px 16px; margin-top: auto; padding-top: 28px; color: rgba(247,242,231,.82); font: 12px 'JetBrains Mono', monospace; letter-spacing: .05em; }
.feature-bottom b { margin-left: auto; color: #e5c187; font-size: 12px; font-weight: 600; }.feature-bottom b i { margin-left: 5px; font-size: 16px; font-style: normal; }

.journal-heading { display: flex; align-items: center; justify-content: space-between; margin-top: 80px; padding-bottom: 16px; border-bottom: 1px solid rgba(25, 54, 66, 0.2); color: #52666f; font-size: 13px; font-weight: 600; scroll-margin-top: 84px; }
.journal-heading p { display: flex; align-items: center; gap: 9px; margin: 0; }.journal-heading p span { width: 19px; height: 1px; background: #b0763b; }.journal-heading > span { color: #60747b; }
.journal-list { min-height: 280px; }

.single-note { display: flex; align-items: center; gap: 15px; padding: 34px 4px; border-bottom: 1px solid rgba(25, 54, 66, 0.15); color: #52666f; font-size: 14px; }.single-note span { color: #92571e; font: 12px 'JetBrains Mono', monospace; }.single-note p { margin: 0; }

.pagination { display: flex; align-items: center; justify-content: center; gap: 22px; margin-top: 42px; font: 12px 'JetBrains Mono', monospace; color: #586c74; }.pagination button { border: 0; background: transparent; color: #234453; cursor: pointer; font: inherit; transition: color .2s ease; }.pagination button:hover:not(:disabled) { color: #b0763b; }.pagination button:disabled { color: #929e9f; cursor: default; }

.first-note { max-width: 650px; padding: 68px 0 72px; }.first-note-seal { display: grid; place-items: center; width: 47px; height: 47px; margin-bottom: 26px; border: 1px solid #c79455; border-radius: 50%; color: #a66c31; font: 27px Georgia, serif; }.first-note h3 { margin: 0; color: #183240; font-size: clamp(28px, 3vw, 42px); line-height: 1.42; letter-spacing: .025em; }.first-note > p:not(.eyebrow) { max-width: 500px; margin: 19px 0 28px; color: #60727a; font-size: 15px; line-height: 1.9; }.first-note button { background: #d7a462; color: #183240; font-weight: 700; }

.explore { padding: 113px 0 126px; background: #e9e5dc; }.explore-top > span { color: rgba(23,50,65,.25); font: 70px/1 'Noto Serif SC', 'Songti SC', serif; }.topic-grid { display: grid; grid-template-columns: repeat(3, 1fr); margin-top: 54px; border-top: 1px solid rgba(23,50,65,.22); }.topic { position: relative; min-height: 238px; display: flex; flex-direction: column; align-items: flex-start; border: 0; border-right: 1px solid rgba(23,50,65,.22); background: transparent; padding: 25px 28px 26px; color: #173241; cursor: pointer; text-align: left; transition: background .25s ease, color .25s ease; }.topic:last-child { border-right: 0; }.topic-index, .topic-en { font-family: 'JetBrains Mono', monospace; }.topic-index { color: #92571e; font-size: 12px; font-weight: 600; }.topic-en { margin-top: auto; color: #52666f; font-size: 12px; font-weight: 600; letter-spacing: .12em; }.topic strong { margin-top: 10px; font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif; font-size: 27px; font-weight: 600; letter-spacing: .045em; }.topic small { margin-top: 8px; color: #566b73; font-size: 13px; }.topic i { position: absolute; right: 27px; bottom: 27px; color: #e5c187; font-size: 23px; font-style: normal; opacity: 0; transform: translate(-8px, 8px); transition: transform .25s ease, opacity .25s ease; }.topic:hover { background: #173241; color: #f7f2e8; }.topic:hover .topic-en, .topic:hover small { color: rgba(247,242,232,.8); }.topic:hover i { opacity: 1; transform: translate(0); }

.community { display: grid; grid-template-columns: 1.05fr .95fr; gap: clamp(50px, 10vw, 148px); padding-top: 127px; padding-bottom: 127px; }.community h2 { margin: 0; color: #183340; font-size: clamp(40px, 4.2vw, 61px); line-height: 1.25; letter-spacing: .05em; }.community h2 em { color: #af7438; font-style: normal; }.community-copy > p:not(.eyebrow) { max-width: 385px; margin: 24px 0 30px; color: #65777e; font-size: 15px; line-height: 1.9; }.community-copy button { padding: 0 19px; background: #173241; color: #f7f3e9; }.community-copy button:hover { transform: translateY(-2px); background: #295263; }
.community-side { align-self: end; padding: 26px 0 0; border-top: 1px solid rgba(25,54,66,.2); }.side-heading { display: flex; align-items: center; gap: 8px; color: #52666f; font-size: 12px; font-weight: 600; }.side-heading i { flex: 1; height: 1px; background: rgba(25,54,66,.18); }.side-heading b { color: #92571e; font-size: 12px; font-weight: 700; }.pulse-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 11px; padding: 30px 0 26px; border-bottom: 1px solid rgba(25,54,66,.14); }.pulse-stats strong { display: block; color: #183340; font: 34px/1 'Noto Serif SC', 'Songti SC', serif; }.pulse-stats span { display: block; margin-top: 7px; color: #52676e; font-size: 13px; }.tags-wrap p { margin: 22px 0 12px; color: #52676e; font: 12px 'JetBrains Mono', monospace; letter-spacing: .1em; }.tags-wrap > div { display: flex; flex-wrap: wrap; gap: 8px; }.tags-wrap button { border: 1px solid rgba(25,54,66,.24); border-radius: 999px; background: transparent; padding: 7px 11px; color: #3f5964; cursor: pointer; font-size: 13px; transition: color .2s ease, border-color .2s ease, background .2s ease; }.tags-wrap button:hover { border-color: #b27639; background: #f0e5d3; color: #92571e; }

@media (max-width: 840px) {
  .hero-content { padding-top: 150px; }.feature-story, .community { grid-template-columns: 1fr; }.feature-story { min-height: 0; }.feature-image { min-height: 310px; }.feature-copy { min-height: 370px; }.community-side { align-self: start; }.topic { min-height: 215px; }.explore, .latest, .community { padding-top: 88px; padding-bottom: 88px; }
}

@media (max-width: 600px) {
  .hero { min-height: 720px; background-position: 58% center; }.hero::after { background: linear-gradient(180deg, rgba(3,16,28,.22), rgba(3,16,28,.67)); }.hero-content { padding-top: 128px; padding-bottom: 56px; }.hero h1 { max-width: 100%; font-size: clamp(34px, 9.5vw, 38px); line-height: 1.26; text-wrap: wrap; overflow-wrap: anywhere; }.hero-copy { font-size: 15px; }.hero-actions { margin-top: 27px; }.hero-footer { padding-bottom: 21px; }.hero-issue em, .scroll-cue span { display: none; }.section-intro, .explore-top { align-items: flex-start; flex-direction: column; gap: 16px; }.section-intro > p { margin-top: 0; font-size: 13px; text-align: left; }.feature-image { min-height: 236px; }.feature-copy { min-height: 348px; padding: 31px 25px; }.feature-copy h3 { font-size: 27px; }.feature-bottom { gap: 8px 12px; }.feature-bottom b { width: 100%; margin-left: 0; margin-top: 5px; }.journal-heading { margin-top: 56px; }.topic-grid { grid-template-columns: 1fr; }.topic, .topic + .topic { min-height: 166px; padding: 21px 0; border-right: 0; border-bottom: 1px solid rgba(23,50,65,.22); }.topic:last-child { border-bottom: 0; }.topic-en { margin-top: 18px; }.topic i { right: 4px; bottom: 25px; opacity: 1; transform: none; }.explore-top > span { display: none; }.community { gap: 46px; }.community h2 { font-size: 42px; }.pulse-stats strong { font-size: 30px; }.first-note { padding: 45px 0 52px; }.first-note h3 { font-size: 29px; }.story-summary { -webkit-line-clamp: 4; }
}

@media (prefers-reduced-motion: reduce) {
  .reveal, .animate-in { animation: none !important; opacity: 1; }
  *, *::before, *::after { scroll-behavior: auto !important; transition-duration: .01ms !important; }
}
</style>
