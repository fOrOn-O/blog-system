<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { createTag, deleteTag, getTags, updateTag } from '@/api/tag'
import { ElMessage, ElMessageBox } from 'element-plus'

const tags = ref([])
const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const dialogMode = ref('create')
const editingTag = ref(null)
const tagName = ref('')
const tagNameInput = ref(null)

const dialogTitle = computed(() => dialogMode.value === 'create' ? '新建标签' : '编辑标签')
const normalizedName = computed(() => tagName.value.trim())
const canSubmit = computed(() => normalizedName.value.length > 0 && normalizedName.value.length <= 50)

async function fetchTags() {
  loading.value = true
  try {
    const res = await getTags()
    const list = res.data || res || []
    tags.value = [...list].sort((a, b) => a.id - b.id)
  } catch (error) {
    console.error('获取标签列表失败:', error)
  } finally {
    loading.value = false
  }
}

function focusNameInput() {
  nextTick(() => tagNameInput.value?.focus())
}

function openCreateDialog() {
  dialogMode.value = 'create'
  editingTag.value = null
  tagName.value = ''
  dialogVisible.value = true
}

function openEditDialog(tag) {
  dialogMode.value = 'edit'
  editingTag.value = tag
  tagName.value = tag.name
  dialogVisible.value = true
}

function closeDialog() {
  if (submitting.value) return
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!canSubmit.value) {
    ElMessage.warning('请输入 1–50 个字符的标签名称')
    return
  }

  const duplicate = tags.value.some(tag => (
    tag.id !== editingTag.value?.id &&
    tag.name.trim().toLocaleLowerCase() === normalizedName.value.toLocaleLowerCase()
  ))

  if (duplicate) {
    ElMessage.warning('标签名称已存在')
    return
  }

  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await createTag({ name: normalizedName.value })
      ElMessage.success('标签创建成功')
    } else {
      await updateTag(editingTag.value.id, { name: normalizedName.value })
      ElMessage.success('标签更新成功')
    }

    dialogVisible.value = false
    await fetchTags()
  } catch (error) {
    console.error(`${dialogMode.value === 'create' ? '创建' : '更新'}标签失败:`, error)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(tag) {
  try {
    await ElMessageBox.confirm(
      `确定删除标签「${tag.name}」吗？此操作无法撤销。`,
      '删除标签',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )

    await deleteTag(tag.id)
    ElMessage.success('标签已删除')
    await fetchTags()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('删除标签失败:', error)
    }
  }
}

onMounted(fetchTags)
</script>

<template>
  <section class="tag-admin-page container animate-in">
    <header class="workspace-header">
      <div>
        <span class="workspace-kicker">ADMIN / TAXONOMY</span>
        <h1>标签管理</h1>
        <p>维护文章使用的公共标签，名称修改会同步影响标签展示。</p>
      </div>

      <button class="create-button" type="button" @click="openCreateDialog">
        <span aria-hidden="true">＋</span>
        新建标签
      </button>
    </header>

    <div class="workspace-status" aria-live="polite">
      <span class="status-label">标签总数</span>
      <strong>{{ tags.length }}</strong>
      <span class="status-note">个可用标签</span>
    </div>

    <div
      v-loading="loading"
      class="tag-table-wrap"
      :aria-busy="loading"
    >
      <table v-if="tags.length" class="tag-table">
        <thead>
          <tr>
            <th scope="col">ID</th>
            <th scope="col">标签名称</th>
            <th scope="col" class="action-heading">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="tag in tags" :key="tag.id">
            <td class="tag-id">#{{ tag.id }}</td>
            <td>
              <span class="tag-name">{{ tag.name }}</span>
            </td>
            <td class="row-actions">
              <button type="button" class="action-button" @click="openEditDialog(tag)">编辑</button>
              <button type="button" class="action-button action-button--danger" @click="handleDelete(tag)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-else-if="!loading" class="empty-state">
        <span aria-hidden="true">#</span>
        <h2>还没有标签</h2>
        <p>创建第一个标签，让文章更容易被发现。</p>
        <button type="button" @click="openCreateDialog">新建标签</button>
      </div>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="min(92vw, 440px)"
      :close-on-click-modal="!submitting"
      :close-on-press-escape="!submitting"
      @opened="focusNameInput"
    >
      <div class="dialog-copy">
        <label for="tag-name">标签名称</label>
        <p>名称将在文章编辑器和文章详情中公开显示。</p>
      </div>
      <el-input
        id="tag-name"
        ref="tagNameInput"
        v-model="tagName"
        maxlength="50"
        show-word-limit
        placeholder="例如：Go、Vue、Redis"
        @keyup.enter="handleSubmit"
      />

      <template #footer>
        <button class="dialog-button dialog-button--ghost" type="button" :disabled="submitting" @click="closeDialog">
          取消
        </button>
        <button class="dialog-button dialog-button--primary" type="button" :disabled="!canSubmit || submitting" @click="handleSubmit">
          {{ submitting ? '保存中…' : (dialogMode === 'create' ? '创建标签' : '保存修改') }}
        </button>
      </template>
    </el-dialog>
  </section>
</template>

<style lang="scss" scoped>
.tag-admin-page {
  max-width: 980px;
  padding-top: 18px;
  padding-bottom: 44px;
}

.workspace-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 32px;
  padding-bottom: 30px;
  border-bottom: 1px solid rgba(25, 50, 65, 0.16);
}

.workspace-kicker {
  display: block;
  margin-bottom: 12px;
  color: #a66d2e;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

h1 {
  margin: 0;
  color: #102332;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: clamp(34px, 5vw, 52px);
  font-weight: 600;
  letter-spacing: -0.04em;
  line-height: 1.08;
}

.workspace-header p {
  max-width: 560px;
  margin: 13px 0 0;
  color: #5e7180;
  font-size: 14px;
}

.create-button,
.empty-state button,
.dialog-button,
.action-button {
  border: 0;
  font: inherit;
  cursor: pointer;
}

.create-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 43px;
  padding: 0 18px;
  border-radius: 999px;
  background: #17384a;
  color: #faf7ef;
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
  transition: transform 0.2s ease, background 0.2s ease;
}

.create-button span {
  margin-right: 6px;
  color: #e4b678;
  font-size: 18px;
  line-height: 1;
}

.create-button:hover {
  background: #244f64;
  transform: translateY(-2px);
}

.workspace-status {
  display: flex;
  align-items: baseline;
  gap: 9px;
  padding: 27px 2px 18px;
}

.status-label,
.status-note,
.tag-id,
.tag-table th {
  font-family: 'JetBrains Mono', monospace;
}

.status-label {
  color: #6a7a86;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.11em;
}

.workspace-status strong {
  color: #102332;
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 30px;
  line-height: 1;
}

.status-note {
  color: #7c8992;
  font-size: 11px;
}

.tag-table-wrap {
  min-height: 260px;
  overflow-x: auto;
  border-top: 2px solid #17384a;
  border-bottom: 1px solid rgba(25, 50, 65, 0.16);
  background: rgba(255, 255, 255, 0.46);
}

.tag-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

.tag-table th,
.tag-table td {
  padding: 17px 20px;
  text-align: left;
  border-bottom: 1px solid rgba(25, 50, 65, 0.1);
}

.tag-table th {
  color: #73818b;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.tag-table th:first-child,
.tag-table td:first-child {
  width: 120px;
}

.tag-table th:last-child,
.tag-table td:last-child {
  width: 190px;
}

.tag-table tbody tr {
  transition: background 0.2s ease, transform 0.2s ease;
}

.tag-table tbody tr:hover {
  background: rgba(228, 182, 120, 0.1);
}

.tag-table tbody tr:last-child td {
  border-bottom: 0;
}

.tag-id {
  color: #81909a;
  font-size: 12px;
}

.tag-name {
  color: #17384a;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: 17px;
  font-weight: 600;
}

.action-heading {
  text-align: right !important;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 7px;
}

.action-button {
  padding: 6px 9px;
  background: transparent;
  color: #31576a;
  font-size: 12px;
  font-weight: 700;
  transition: color 0.18s ease, background 0.18s ease;
}

.action-button:hover {
  border-radius: 6px;
  background: rgba(49, 87, 106, 0.08);
  color: #102332;
}

.action-button--danger {
  color: #ad4455;
}

.action-button--danger:hover {
  background: rgba(173, 68, 85, 0.08);
  color: #8d2638;
}

.empty-state {
  display: grid;
  place-items: center;
  min-height: 300px;
  padding: 48px 20px;
  text-align: center;
}

.empty-state > span {
  color: #d6a261;
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 46px;
  line-height: 1;
}

.empty-state h2 {
  margin: 13px 0 4px;
  color: #17384a;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: 21px;
}

.empty-state p {
  margin: 0 0 18px;
  color: #71808b;
  font-size: 13px;
}

.empty-state button {
  padding: 9px 14px;
  border-bottom: 1px solid #a66d2e;
  background: transparent;
  color: #8b5b25;
  font-size: 13px;
  font-weight: 700;
}

.dialog-copy {
  margin-bottom: 12px;
}

.dialog-copy label {
  color: #17384a;
  font-size: 14px;
  font-weight: 700;
}

.dialog-copy p {
  margin: 4px 0 0;
  color: #71808b;
  font-size: 12px;
}

.dialog-button {
  min-height: 38px;
  padding: 0 16px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 700;
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.dialog-button:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.dialog-button--ghost {
  background: transparent;
  color: #5d6c77;
}

.dialog-button--primary {
  margin-left: 7px;
  background: #17384a;
  color: #faf7ef;
}

.dialog-button--primary:not(:disabled):hover {
  transform: translateY(-1px);
}

button:focus-visible {
  outline: 2px solid #d7a462;
  outline-offset: 3px;
}

:deep(.el-dialog) {
  border-radius: 16px;
  overflow: hidden;
}

:deep(.el-dialog__title) {
  color: #17384a;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, serif;
  font-size: 21px;
  font-weight: 600;
}

@media (max-width: 680px) {
  .tag-admin-page {
    padding-top: 4px;
  }

  .workspace-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 22px;
  }

  .create-button {
    width: 100%;
  }

  .tag-table th,
  .tag-table td {
    padding: 15px 12px;
  }

  .tag-table th:first-child,
  .tag-table td:first-child {
    width: 72px;
  }

  .tag-table th:last-child,
  .tag-table td:last-child {
    width: 132px;
  }

  .row-actions {
    gap: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .tag-admin-page,
  .create-button,
  .tag-table tbody tr,
  .dialog-button {
    animation: none;
    transition: none;
  }
}
</style>
