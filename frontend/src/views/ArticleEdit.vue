<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getArticle, createArticle, updateArticle } from '@/api/article'
import { getTags } from '@/api/tag'
import { uploadImage } from '@/api/upload'
import { ElMessage } from 'element-plus'
import RichTextEditor from '@/components/RichTextEditor.vue'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.params.id)
const articleId = computed(() => route.params.id)

const form = ref({
  title: '',
  content: '',
  summary: '',
  cover_image: '',
  tag_ids: []
})

const loading = ref(false)
const submitting = ref(false)
const uploading = ref(false)

// 标签相关
const tags = ref([])

// 获取文章详情（编辑模式）
async function fetchArticle() {
  if (!isEdit.value) return

  loading.value = true
  try {
    const res = await getArticle(articleId.value)
    const article = res.data
    form.value = {
      title: article.title,
      content: article.content || '',
      summary: article.summary || '',
      cover_image: article.cover_image || '',
      tag_ids: article.tags?.map(t => t.id) || []
    }
  } catch (error) {
    console.error('获取文章失败:', error)
    ElMessage.error('文章不存在')
    router.push('/')
  } finally {
    loading.value = false
  }
}

// 获取所有标签
async function fetchTags() {
  try {
    const res = await getTags()
    tags.value = res.data || res || []
  } catch (error) {
    console.error('获取标签失败:', error)
  }
}

// 上传封面图片
async function handleCoverUpload(event) {
  const file = event.target.files[0]
  if (!file) return

  // 检查文件大小（10MB）
  if (file.size > 10 * 1024 * 1024) {
    ElMessage.warning('图片大小不能超过10MB')
    return
  }

  // 检查文件类型
  const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']
  if (!allowedTypes.includes(file.type)) {
    ElMessage.warning('只支持 jpg、png、gif、webp 格式')
    return
  }

  uploading.value = true
  try {
    const res = await uploadImage(file)
    const data = res.data || res
    form.value.cover_image = data.url
    ElMessage.success('封面上传成功')
  } catch (error) {
    console.error('上传失败:', error)
    ElMessage.error('上传失败')
  } finally {
    uploading.value = false
    // 清空input
    event.target.value = ''
  }
}

// 删除封面
function removeCover() {
  form.value.cover_image = ''
}

// 提交文章
async function handleSubmit() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请输入文章标题')
    return
  }

  const contentText = form.value.content
    .replace(/<[^>]*>/g, '')
    .replace(/&nbsp;|&#160;/gi, ' ')
    .trim()
  const hasImage = /<img\b/i.test(form.value.content)
  if (!contentText && !hasImage) {
    ElMessage.warning('请输入文章内容')
    return
  }

  submitting.value = true
  try {
    if (isEdit.value) {
      await updateArticle(articleId.value, form.value)
      ElMessage.success('更新成功')
    } else {
      const res = await createArticle(form.value)
      ElMessage.success('发布成功')
      router.push(`/article/${res.data.id}`)
      return
    }
    router.push(`/article/${articleId.value}`)
  } catch (error) {
    console.error('提交失败:', error)
  } finally {
    submitting.value = false
  }
}

// 取消
function handleCancel() {
  router.back()
}

onMounted(() => {
  fetchArticle()
  fetchTags()
})
</script>

<template>
  <div v-loading="loading" class="article-edit-page container">
    <div class="page-header">
      <h1 class="page-title">{{ isEdit ? '编辑文章' : '写文章' }}</h1>
    </div>

    <div class="edit-form card">
      <el-form :model="form" label-position="top">
        <!-- 文章标题 -->
        <el-form-item label="文章标题" required>
          <el-input
            v-model="form.title"
            placeholder="请输入文章标题"
            maxlength="100"
            show-word-limit
          />
        </el-form-item>

        <!-- 文章摘要 -->
        <el-form-item label="文章摘要">
          <el-input
            v-model="form.summary"
            type="textarea"
            :rows="3"
            placeholder="请输入文章摘要（选填，不填则自动截取）"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>

        <!-- 文章标签 -->
        <el-form-item label="文章标签">
          <el-select
            v-model="form.tag_ids"
            multiple
            filterable
            placeholder="选择标签（可多选）"
            style="width: 100%"
          >
            <el-option
              v-for="tag in tags"
              :key="tag.id"
              :label="tag.name"
              :value="tag.id"
            />
          </el-select>
          <div class="tag-hint">
            <span class="hint-text">选择合适的文章标签，方便读者分类浏览</span>
          </div>
        </el-form-item>

        <!-- 封面图片 -->
        <el-form-item label="封面图片">
          <div class="cover-upload-area">
            <div v-if="form.cover_image" class="cover-preview">
              <img :src="form.cover_image" alt="封面预览">
              <button class="remove-btn" @click="removeCover" type="button">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
            <div v-else class="cover-upload-btn">
              <input
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp"
                @change="handleCoverUpload"
                id="cover-input"
                class="file-input"
              />
              <label for="cover-input" class="upload-label">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>
                </svg>
                <span>{{ uploading ? '上传中...' : '点击上传封面' }}</span>
                <span class="upload-hint">支持 jpg、png、gif、webp，最大 10MB</span>
              </label>
            </div>
          </div>
        </el-form-item>

        <!-- 文章内容 -->
        <el-form-item label="文章内容" required>
          <RichTextEditor v-model="form.content" />
        </el-form-item>

        <!-- 提交按钮 -->
        <el-form-item>
          <div class="form-actions">
            <el-button @click="handleCancel">取消</el-button>
            <el-button
              type="primary"
              :loading="submitting"
              @click="handleSubmit"
            >
              {{ isEdit ? '保存修改' : '发布文章' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.article-edit-page {
  padding-top: 20px;
  padding-bottom: 40px;
  max-width: 800px;
}

.edit-form {
  :deep(.el-form-item__label) {
    font-weight: 500;
  }
}

// ── 标签选择 ───────────────────────────────────────────
.tag-hint {
  margin-top: 8px;

  .hint-text {
    font-size: 13px;
    color: var(--text-muted);
  }
}

// ── 封面上传 ───────────────────────────────────────────
.cover-upload-area {
  width: 100%;
}

.cover-preview {
  position: relative;
  max-width: 400px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #E2E8F0;

  img {
    width: 100%;
    height: auto;
    display: block;
  }

  .remove-btn {
    position: absolute;
    top: 8px;
    right: 8px;
    width: 28px;
    height: 28px;
    background: rgba(0, 0, 0, 0.6);
    color: white;
    border: none;
    border-radius: 50%;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s ease;

    &:hover {
      background: rgba(239, 68, 68, 0.8);
    }
  }
}

.cover-upload-btn {
  .file-input {
    display: none;
  }

  .upload-label {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 32px;
    background: #F7FAFC;
    border: 2px dashed #E2E8F0;
    border-radius: 8px;
    cursor: pointer;
    transition: color 0.15s ease, border-color 0.15s ease, background-color 0.15s ease;
    color: var(--text-muted);

    &:hover {
      border-color: #3B68CC;
      color: #3B68CC;
      background: #EBF4FF;
    }

    span {
      font-size: 14px;
    }

    .upload-hint {
      font-size: 13px;
      color: var(--text-muted);
    }
  }
}

// ── 表单操作 ───────────────────────────────────────────
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

// ── 响应式 ─────────────────────────────────────────────
@media (max-width: 768px) {
  .article-edit-page {
    padding-left: 12px;
    padding-right: 12px;
  }

  .cover-preview {
    max-width: 100%;
  }
}
</style>
