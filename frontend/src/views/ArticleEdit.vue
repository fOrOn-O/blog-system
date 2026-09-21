<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { getOwnedArticle, createArticle, updateArticle, createDraft, saveDraft, publishArticle, archiveArticle, getArticleVersions, getArticleVersion, getVersionDiff } from '@/api/article'
import { getTags } from '@/api/tag'
import { uploadImage } from '@/api/upload'
import { ElMessage, ElMessageBox } from 'element-plus'
import { articleLoadError } from '@/utils/article-navigation'
import RichTextEditor from '@/components/RichTextEditor.vue'
import AgentAssistantPanel from '@/components/AgentAssistantPanel.vue'
import ArticleDiff from '@/components/ArticleDiff.vue'
import { sanitizeArticleHTML } from '@/utils/article-html'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.params.id)
const articleId = computed(() => route.params.id)

// 保存开始编辑时看到的工作版本，发生冲突时保留原值和用户输入。
const expectedVersion = ref(null)
const currentArticle = ref(null)
const versions = ref([])
const historyPage = ref(1)
const historyTotal = ref(0)
const historyDiff = ref(null)
const assistantBusy = ref(false)
const switching = ref(false)
const editorUploading = ref(false)
const loadError = ref('')
const baseline = ref('')
let loadSequence = 0
let savedDestination = null
const workspace = computed(() => expectedVersion.value && currentArticle.value ? { article_id: Number(articleId.value), version_no: expectedVersion.value } : null)
const dirty = computed(() => !!baseline.value && JSON.stringify(form.value) !== baseline.value)
const readOnlyVersion = computed(() => currentArticle.value && expectedVersion.value !== currentArticle.value.version)
const uploadPending = computed(() => uploading.value || editorUploading.value)
const workspaceBusy = computed(() => loading.value || submitting.value || assistantBusy.value || switching.value || uploadPending.value)
const formLocked = computed(() => workspaceBusy.value || !!loadError.value || readOnlyVersion.value || currentArticle.value?.status === 'archived')
const versionOptions = computed(() => [...new Set([expectedVersion.value, currentArticle.value?.version, ...versions.value.map(v => v.version_no)].filter(Boolean))].sort((a, b) => b-a))

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
function setEditor(article) {
  form.value = {
    title: article.title, content: sanitizeArticleHTML(article.content || ''), summary: article.summary || '',
    cover_image: article.cover_image || '', tag_ids: currentArticle.value?.tags?.map(t => t.id) || []
  }
  baseline.value = JSON.stringify(form.value)
}

async function fetchArticle(targetVersion = null) {
  if (!isEdit.value) return
  const id = articleId.value
  const seq = ++loadSequence
  loading.value = true
  loadError.value = ''
  try {
    const [res, history] = await Promise.all([getOwnedArticle(id, { notifyError: false }), getArticleVersions(id, 1, { notifyError: false })])
    const snapshot = targetVersion && targetVersion !== res.data.version ? await getArticleVersion(id, targetVersion, { notifyError: false }) : null
    if (seq !== loadSequence || id !== articleId.value) return
    const article = res.data
    currentArticle.value = article
    expectedVersion.value = targetVersion || article.version
    versions.value = history.data
    historyPage.value = 1
    historyTotal.value = history.meta.total
    historyDiff.value = null
    setEditor(snapshot?.data || article)
  } catch (error) {
    if (seq === loadSequence) loadError.value = articleLoadError(error)
  } finally {
    if (seq === loadSequence) loading.value = false
  }
}

async function confirmDiscardLocal() {
  if (!dirty.value) return true
  try { await ElMessageBox.confirm('此操作会放弃未保存的本地修改，继续吗？', '未保存修改', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' }); return true } catch { return false }
}
async function switchWorkspace(version = null) {
  if (workspaceBusy.value) return
  switching.value = true
  try { if (await confirmDiscardLocal()) await fetchArticle(version) }
  finally { switching.value = false }
}
async function refreshArticle() { await switchWorkspace() }
async function selectVersion(event) {
  const version = Number(event.target.value)
  event.target.value = expectedVersion.value
  await switchWorkspace(version)
}
async function moreVersions() {
  const seq = loadSequence
  const result = await getArticleVersions(articleId.value, historyPage.value + 1)
  if (seq !== loadSequence) return
  versions.value.push(...result.data); historyPage.value++
}
async function compareVersions() {
  if (expectedVersion.value <= 1) return
  const seq = loadSequence
  try {
    const result = await getVersionDiff(articleId.value, expectedVersion.value - 1, expectedVersion.value)
    if (seq === loadSequence) historyDiff.value = result.data
  } catch { /* API 层显示错误，保留编辑内容。 */ }
}
async function applied(result) {
  if (result.article_id !== Number(articleId.value)) return
  ElMessage.success(`已保存工作版本 V${result.new_version_no}，未自动发布`)
  await fetchArticle(result.new_version_no)
}
async function checkCurrentVersion() {
  const id = articleId.value
  const seq = loadSequence
  try { const result = await getOwnedArticle(id); if (id === articleId.value && seq === loadSequence) currentArticle.value = result.data } catch { /* 保留冲突状态。 */ }
}
function setAssistantBusy(value) {
  assistantBusy.value = value
  if (value) loadSequence++
}
async function lifecycle(action) {
  if (formLocked.value || dirty.value || !currentArticle.value) return
  const target = { id: articleId.value, version: expectedVersion.value }
  submitting.value = true
  loadSequence++
  try {
    await ElMessageBox.confirm(action === 'publish' ? `公开工作版本 V${expectedVersion.value}？` : '归档后文章不再公开，且不能继续编辑。', action === 'publish' ? '确认发布' : '确认归档', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' })
    if (articleId.value !== target.id || expectedVersion.value !== target.version || dirty.value) return
    if (action === 'publish') await publishArticle(target.id, target.version)
    else await archiveArticle(target.id)
    await fetchArticle()
  } catch { /* 取消或业务错误均不自动重试。 */ } finally { submitting.value = false }
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
  if (formLocked.value) return
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
async function handleSubmit(draft = false) {
  if (formLocked.value || uploading.value) return
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
  loadSequence++
  try {
    if (isEdit.value) {
      await (draft ? saveDraft : updateArticle)(articleId.value, {
        ...form.value,
        expected_version: expectedVersion.value
      })
      ElMessage.success(draft ? '草稿已保存，未发布' : '更新成功')
    } else {
      const res = await (draft ? createDraft : createArticle)(form.value)
      baseline.value = JSON.stringify(form.value)
      ElMessage.success(draft ? '草稿已保存' : '发布成功')
      if (draft) {
        savedDestination = `/article/edit/${res.data.id}`
        await router.replace(savedDestination)
        return
      }
      router.push(`/article/${res.data.id}`)
      return
    }
    if (draft) await fetchArticle()
    else { baseline.value = JSON.stringify(form.value); router.push(`/article/${articleId.value}`) }
  } catch (error) {
    console.error('提交失败:', error)
  } finally {
    submitting.value = false
    savedDestination = null
  }
}

// 取消
function handleCancel() {
  router.back()
}

watch(articleId, () => {
  loadSequence++
  loading.value = false; loadError.value = ''
  expectedVersion.value = null; currentArticle.value = null; versions.value = []; historyDiff.value = null; baseline.value = ''
  if (isEdit.value) fetchArticle()
  else {
    form.value = { title: '', content: '', summary: '', cover_image: '', tag_ids: [] }
    baseline.value = JSON.stringify(form.value)
  }
}, { immediate: true })
onBeforeRouteLeave(async () => !assistantBusy.value && !submitting.value && !uploadPending.value && !switching.value && await confirmDiscardLocal())
onBeforeRouteUpdate(async (to, from) => to.path === savedDestination || to.params.id === from.params.id || (!assistantBusy.value && !submitting.value && !uploadPending.value && !switching.value && await confirmDiscardLocal()))
onMounted(fetchTags)
</script>

<template>
  <div v-loading="loading" class="article-edit-page container">
    <div class="page-header">
      <h1 class="page-title">{{ isEdit ? '编辑文章' : '写文章' }}</h1>
    </div>

    <p v-if="loadError" role="alert">{{ loadError }} <el-button @click="refreshArticle">重新加载</el-button></p>
    <div v-if="currentArticle" class="version-bar card">
      <span data-testid="article-version">当前工作版本 V{{ currentArticle.version }} · 公开版本 {{ currentArticle.published_version ? `V${currentArticle.published_version}` : '无' }} · {{ currentArticle.status }}</span>
      <label>活动版本 <select aria-label="活动版本" :value="expectedVersion" :disabled="workspaceBusy" @change="selectVersion"><option v-for="version in versionOptions" :key="version" :value="version">V{{ version }}</option></select></label>
      <el-button v-if="versions.length < historyTotal" @click="moreVersions">加载更早版本</el-button>
      <el-button :disabled="expectedVersion <= 1 || loading || assistantBusy" @click="compareVersions">与上一版比较</el-button>
      <el-button :disabled="workspaceBusy" @click="refreshArticle">刷新工作版本</el-button>
      <p v-if="readOnlyVersion">正在查看历史版本，编辑只在最新工作版本中进行。</p>
      <ArticleDiff v-if="historyDiff" :diff="historyDiff" />
    </div>
    <div class="workspace-layout">
    <fieldset :disabled="formLocked" class="edit-form card">
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
          <RichTextEditor v-model="form.content" :disabled="!!formLocked" @uploading="editorUploading = $event" />
        </el-form-item>

        <!-- 提交按钮 -->
        <el-form-item>
          <div class="form-actions">
            <el-button @click="handleCancel">取消</el-button>
            <el-button :disabled="!!formLocked" :loading="submitting" @click="handleSubmit(true)">保存草稿</el-button>
            <el-button v-if="isEdit" :disabled="!!formLocked || dirty" @click="lifecycle('publish')">发布工作版本</el-button>
            <el-button v-if="isEdit" :disabled="!!formLocked || dirty" @click="lifecycle('archive')">归档文章</el-button>
            <el-button
              type="primary"
              :loading="submitting"
              :disabled="!!formLocked"
              @click="handleSubmit(false)"
            >
              {{ isEdit ? '保存并发布' : '发布文章' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
    </fieldset>
    <AgentAssistantPanel :workspace="workspace" :current-version="currentArticle?.version" :dirty="dirty" :disabled="loading || submitting || switching || uploadPending || !!loadError || currentArticle?.status === 'archived'" @busy="setAssistantBusy" @applied="applied" @refresh="refreshArticle" @conflict="checkCurrentVersion" />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.article-edit-page {
  padding-top: 20px;
  padding-bottom: 40px;
  max-width: 1440px;
}
.workspace-layout { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(340px, 1fr); gap: 24px; }
.edit-form { min-width: 0; border: 0; margin: 0; }
.version-bar { padding: 16px; margin-bottom: 20px; display: flex; flex-wrap: wrap; align-items: center; gap: 12px; font-size: 14px; }
.version-bar .article-diff { flex-basis: 100%; }
@media (max-width: 1000px) { .workspace-layout { grid-template-columns: minmax(0, 1fr); } }

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
  flex-wrap: wrap;
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
