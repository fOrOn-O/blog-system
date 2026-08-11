<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { searchArticles } from '@/api/article'
import ArticleCard from '@/components/ArticleCard.vue'

const route = useRoute()

const articles = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const keyword = ref('')

// 搜索文章
async function fetchArticles() {
  if (!keyword.value.trim()) return

  loading.value = true
  try {
    const res = await searchArticles({
      keyword: keyword.value.trim(),
      page: currentPage.value,
      page_size: pageSize.value
    })
    articles.value = res.data || res || []
    total.value = res.total || res.meta?.total || 0
  } catch (error) {
    console.error('搜索失败:', error)
  } finally {
    loading.value = false
  }
}

// 页码变化
function handlePageChange(page) {
  currentPage.value = page
  fetchArticles()
}

// 每页数量变化
function handleSizeChange(size) {
  pageSize.value = size
  currentPage.value = 1
  fetchArticles()
}

// 监听路由参数变化
watch(
  () => route.query.keyword,
  (newKeyword) => {
    if (newKeyword) {
      keyword.value = newKeyword
      currentPage.value = 1
      fetchArticles()
    }
  },
  { immediate: true }
)

onMounted(() => {
  if (route.query.keyword) {
    keyword.value = route.query.keyword
    fetchArticles()
  }
})
</script>

<template>
  <div class="search-page container">
    <div class="page-header">
      <h1 class="page-title">
        搜索结果：
        <span class="keyword">"{{ keyword }}"</span>
      </h1>
      <span class="result-count">共 {{ total }} 篇文章</span>
    </div>

    <div v-loading="loading" class="search-results">
      <template v-if="articles.length > 0">
        <ArticleCard
          v-for="article in articles"
          :key="article.id"
          :article="article"
        />

        <!-- 分页 -->
        <div class="pagination-wrapper">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next, jumper"
            @current-change="handlePageChange"
            @size-change="handleSizeChange"
          />
        </div>
      </template>

      <el-empty v-else description="未找到相关文章" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.search-page {
  padding-top: 20px;
  padding-bottom: 40px;
}

.page-header {
  display: flex;
  align-items: baseline;
  gap: 16px;

  .keyword {
    color: #409eff;
  }

  .result-count {
    font-size: 14px;
    color: #909399;
  }
}

.search-results {
  min-height: 400px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 24px;
  padding: 20px 0;
}
</style>
