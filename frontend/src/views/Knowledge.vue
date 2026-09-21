<script setup>
import { onBeforeUnmount, ref } from 'vue'
import { askKnowledge } from '@/api/knowledge'
import KnowledgeSources from '@/components/KnowledgeSources.vue'
import AssistantMarkdown from '@/components/AssistantMarkdown.vue'
import { assistantErrorMessage } from '@/api/assistant-error'

const query = ref('')
const loading = ref(false)
const result = ref(null)
const error = ref('')
let active = true
onBeforeUnmount(() => { active = false })
async function ask() {
  if (loading.value || !query.value.trim()) return
  loading.value = true
  error.value = ''
  result.value = null
  try {
    const answer = await askKnowledge(query.value.trim())
    if (active) result.value = answer
  } catch (failure) {
    if (active) error.value = assistantErrorMessage(failure)
  } finally { if (active) loading.value = false }
}
</script>

<template>
  <main class="knowledge-page container">
    <h1>站内知识助手</h1>
    <p>基于本站当前所有已发布文章的知识问答助手</p>
    <p class="hint">每次提问独立检索。请写出完整问题，回答仅使用检索到的文章依据。</p>
    <form class="card question-form" @submit.prevent="ask">
      <label for="knowledge-question">你的问题</label>
      <textarea id="knowledge-question" v-model="query" rows="4" maxlength="12000" :disabled="loading" placeholder="例如：这些文章介绍了哪些缓存持久化方式？" />
      <el-button native-type="submit" type="primary" :loading="loading" :disabled="loading || !query.trim()">提问</el-button>
    </form>
    <p v-if="loading" role="status">正在检索已发布文章并整理回答…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <section v-if="result" class="card answer" aria-label="知识回答">
      <h2>{{ result.has_evidence ? '基于文章的回答' : '暂无足够依据' }}</h2>
      <AssistantMarkdown :text="result.answer" />
      <KnowledgeSources :sources="result.sources" />
    </section>
  </main>
</template>

<style scoped>
.knowledge-page { max-width: 900px; padding-top: 36px; padding-bottom: 60px; }
h1 { margin-bottom: 14px; } .hint { color: var(--text-muted); line-height: 1.7; }
.question-form { display: grid; gap: 14px; padding: 24px; margin: 24px 0; }
textarea { width: 100%; box-sizing: border-box; border: 1px solid #ccd5e0; border-radius: 8px; padding: 12px; font: inherit; resize: vertical; }
.question-form .el-button { justify-self: end; } .answer { padding: 24px; }
.answer-text { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.9; } [role=alert] { color: #a83737; }
</style>
