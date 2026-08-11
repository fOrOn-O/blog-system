<script setup>
import { ref, onMounted } from 'vue'
import { getFavorites } from '@/api/article'
import ArticleCard from '@/components/ArticleCard.vue'

const favorites = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 获取收藏列表
async function fetchFavorites() {
  loading.value = true
  try {
    const res = await getFavorites({
      page: currentPage.value,
      page_size: pageSize.value
    })
    // 收藏接口返回格式: { data: [{ id, article: {...} }] }
    // 需要转换为文章格式
    const list = res.data || res || []
    favorites.value = list.map(item => item.article || item)
    total.value = res.total || res.meta?.total || 0
  } catch (error) {
    console.error('获取收藏列表失败:', error)
  } finally {
    loading.value = false
  }
}

// 页码变化
function handlePageChange(page) {
  currentPage.value = page
  fetchFavorites()
}

// 每页数量变化
function handleSizeChange(size) {
  pageSize.value = size
  currentPage.value = 1
  fetchFavorites()
}

onMounted(() => {
  fetchFavorites()
})
</script>

<template>
  <div class="favorites-page container">
    <div class="page-header">
      <h1 class="page-title">我的收藏</h1>
    </div>

    <div v-loading="loading" class="favorites-list">
      <template v-if="favorites.length > 0">
        <ArticleCard
          v-for="(article, index) in favorites"
          :key="article.id"
          :article="article"
          :index="(currentPage - 1) * pageSize + index"
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

      <el-empty v-else description="暂无收藏文章" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.favorites-page {
  padding-top: 20px;
  padding-bottom: 40px;
}

.favorites-list {
  min-height: 400px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 24px;
  padding: 20px 0;
}
</style>
