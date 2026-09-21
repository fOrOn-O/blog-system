<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import { chatWithWorkspace } from '@/api/agent'
import { previewEditProposal, applyEditProposal } from '@/api/article'
import ArticleDiff from './ArticleDiff.vue'
import KnowledgeSources from './KnowledgeSources.vue'
import AssistantMarkdown from './AssistantMarkdown.vue'
import { assistantErrorMessage } from '@/api/assistant-error'

const props = defineProps({ workspace: Object, currentVersion: Number, dirty: Boolean, disabled: Boolean })
const emit = defineEmits(['applied', 'refresh', 'busy', 'conflict'])
const message = ref('')
const mode = ref('question')
const messages = ref([])
const expanded = ref(false)
const messageList = ref(null)
const composerInput = ref(null)
let following = true
function trackScroll() {
  const el = messageList.value
  if (el) following = el.scrollHeight - el.scrollTop - el.clientHeight < 40
}
async function scrollMessages() {
  await nextTick()
  if (following && messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight
}
function resizeInput() {
  const el = composerInput.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 140)}px`
}
watch(message, () => nextTick(resizeInput))
onMounted(resizeInput)
const sending = ref(false)
const chatError = ref('')
const proposal = ref(null)
const diff = ref(null)
const phase = ref('NONE')
const proposalError = ref('')
const previewedProposal = ref(null)
watch(() => [messages.value.length, sending.value, chatError.value, expanded.value], scrollMessages, { flush: 'post' })
let revision = 0
const matches = computed(() => proposal.value && props.workspace && proposal.value.article_id === props.workspace.article_id && proposal.value.base_version_no === props.workspace.version_no)
const stale = computed(() => matches.value && props.currentVersion !== proposal.value.base_version_no)
const busy = computed(() => ['CONFIRMING', 'APPLYING'].includes(phase.value))
const canApply = computed(() => phase.value === 'PREVIEWED' && previewedProposal.value === proposal.value && matches.value && !stale.value && !props.dirty && !props.disabled)

// 工作区切换或刷新后保留原提案身份，但旧预览及其迟到响应不再代表本次审核。
watch(() => props.workspace, () => {
  revision++
  previewedProposal.value = null
  diff.value = null
  if (['PREVIEWED', 'PREVIEWING'].includes(phase.value)) phase.value = 'GENERATED'
})

function discard() {
  if (busy.value) return
  revision++
  proposal.value = null
  previewedProposal.value = null
  diff.value = null
  proposalError.value = ''
  phase.value = 'NONE'
}

async function send() {
  if (sending.value || !message.value.trim() || !props.workspace || props.disabled || proposal.value) return
  const workspace = { ...props.workspace }
  const text = message.value.trim()
  messages.value.push({ role: '你', text, workspace })
  message.value = ''
  sending.value = true
  chatError.value = ''
  try {
    const result = await chatWithWorkspace(text, workspace, mode.value)
    messages.value.push({ role: '助手', text: result.answer, workspace, sources: result.sources })
    if (result.proposal) {
      if (result.proposal.article_id !== workspace.article_id || result.proposal.base_version_no !== workspace.version_no) throw new Error('workspace mismatch')
      // 保存响应自己的身份，后续切换文章/版本绝不改写。
      proposal.value = Object.freeze({ ...result.proposal, change_summary: [...result.proposal.change_summary] })
      phase.value = 'GENERATED'
      revision++
    }
  } catch (error) {
    chatError.value = assistantErrorMessage(error)
  } finally { sending.value = false }
}

async function preview() {
  if (!proposal.value || busy.value || phase.value === 'PREVIEWING' || props.disabled) return
  const seq = ++revision
  const captured = proposal.value
  phase.value = 'PREVIEWING'
  previewedProposal.value = null
  diff.value = null
  proposalError.value = ''
  try {
    const result = await previewEditProposal(captured)
    if (seq !== revision) return
    if (result.data.article_id !== captured.article_id || result.data.base_version_no !== captured.base_version_no) throw new Error('preview mismatch')
    diff.value = result.data
    previewedProposal.value = captured
    phase.value = 'PREVIEWED'
  } catch {
    if (seq !== revision) return
    phase.value = 'ERROR'
    proposalError.value = '预览失败，未批准或保存任何修改。请重新预览。'
  }
}

async function apply() {
  if (!canApply.value) return
  const captured = proposal.value
  phase.value = 'CONFIRMING'
  emit('busy', true)
  try {
    await ElMessageBox.confirm(`批准文章 #${captured.article_id} 基于 V${captured.base_version_no} 的修改？将创建新的工作/草稿版本，不会自动发布。`, '确认应用修改', {
      confirmButtonText: '确认应用', cancelButtonText: '取消', type: 'warning', closeOnClickModal: false
    })
    // 确认期间工作区或本地编辑可能变化，不能仅依赖按钮先前状态。
    if (!matches.value || stale.value || props.dirty || props.disabled || proposal.value !== captured || previewedProposal.value !== captured) {
      phase.value = previewedProposal.value === captured ? 'PREVIEWED' : 'GENERATED'
      return
    }
    phase.value = 'APPLYING'
    const result = await applyEditProposal(captured)
    phase.value = 'APPLIED'
    proposal.value = null
    previewedProposal.value = null
    diff.value = null
    proposalError.value = ''
    revision++
    emit('applied', result.data)
  } catch (error) {
    if (phase.value === 'CONFIRMING') {
      phase.value = previewedProposal.value === captured ? 'PREVIEWED' : 'GENERATED'
    } else if (error.response?.status === 409) {
      phase.value = 'CONFLICT'
      proposalError.value = error.response.data?.message?.includes('归档')
        ? '文章已归档，不能应用提案。请刷新文章。'
        : '提案已过期：文章在生成提案后发生变化。请刷新文章并重新生成提案。'
      emit('conflict')
    } else {
      phase.value = 'ERROR'
      diff.value = null
      proposalError.value = '应用未确认成功，请先刷新文章核实结果，再决定是否重新生成提案。不会自动重试。'
    }
  } finally { emit('busy', false) }
}
</script>

<template>
  <aside class="agent-panel card" :class="{ expanded }" aria-label="Agent 助手">
    <header><h2>Agent 助手</h2><span v-if="workspace" data-testid="agent-workspace">文章 #{{ workspace.article_id }} · V{{ workspace.version_no }}</span><button type="button" class="expand-button" :aria-expanded="expanded" aria-controls="agent-messages" @click="expanded = !expanded">{{ expanded ? '收起助手' : '展开助手' }}</button></header>
    <p class="hint">基于已保存版本问答、检索或提出修改建议。应用前需要你预览并确认。</p>
    <p v-if="!workspace" class="hint">先保存文章草稿，再使用助手。</p>
    <p v-if="dirty" class="notice">有未保存修改，助手只读取已保存版本。请保存或撤销本地修改后再应用提案。</p>
    <div id="agent-messages" ref="messageList" class="messages" aria-live="polite" tabindex="0" aria-label="助手消息" @scroll="trackScroll">
      <article v-for="(item, index) in messages" :key="index" class="chat-message">
        <small>{{ item.role }} · #{{ item.workspace.article_id }} / V{{ item.workspace.version_no }}</small>
        <AssistantMarkdown v-if="item.role === '助手'" :text="item.text" />
        <p v-else>{{ item.text }}</p>
        <KnowledgeSources :sources="item.sources || []" :public-links="false" />
      </article>
      <p v-if="sending">助手正在处理…</p><p v-if="chatError" role="alert">{{ chatError }}</p>
    </div>
    <section v-if="proposal" class="proposal" aria-label="编辑提案">
      <h3>编辑提案 · 基于 V{{ proposal.base_version_no }}</h3>
      <small>文章 #{{ proposal.article_id }}</small>
      <ul><li v-for="(summary, index) in proposal.change_summary" :key="index">{{ summary }}</li></ul>
      <p v-if="!matches" class="notice">此提案属于其他文章或版本，无法在当前工作区应用。可切回原版本或丢弃。</p>
      <p v-else-if="stale" class="notice">提案基础 V{{ proposal.base_version_no }}，当前文章已为 V{{ currentVersion }}。请重新生成。</p>
      <p v-if="proposalError" role="alert">{{ proposalError }}</p>
      <ArticleDiff v-if="diff" :diff="diff" />
      <div class="proposal-actions">
        <el-button :disabled="busy || disabled || phase === 'CONFLICT'" :loading="phase === 'PREVIEWING'" @click="preview">预览修改</el-button>
        <el-button :disabled="busy" @click="discard">丢弃提案</el-button>
        <el-button type="primary" :disabled="!canApply" :loading="phase === 'APPLYING'" @click="apply">应用修改</el-button>
        <el-button v-if="phase === 'CONFLICT' || phase === 'ERROR' || stale" :disabled="busy || disabled" @click="emit('refresh')">刷新文章</el-button>
      </div>
    </section>
    <form class="composer" @submit.prevent="send">
      <label>助手模式 <select v-model="mode" aria-label="助手模式" :disabled="sending || !!proposal"><option value="question">版本问答</option><option value="write">写作提案</option></select></label>
      <label for="agent-message">向助手提问</label>
      <textarea id="agent-message" ref="composerInput" v-model="message" rows="3" maxlength="12000" :disabled="sending || disabled || !!proposal || !workspace" :placeholder="mode === 'question' ? '询问当前已保存版本中的内容' : '例如：改写介绍段，保留其他内容'" />
      <p v-if="proposal" class="hint">先处理或丢弃当前提案，再发送新消息。</p>
      <el-button native-type="submit" type="primary" :loading="sending" :disabled="sending || disabled || !!proposal || !workspace || !message.trim()">发送</el-button>
    </form>
  </aside>
</template>

<style scoped>
.agent-panel { padding: 20px; min-width: 0; align-self: start; position: sticky; top: 80px; }
header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; }
h2 { font-size: 19px; margin: 0; } h3 { font-size: 16px; }
small, .hint { color: var(--text-muted); font-size: 12px; }
.hint { margin: 12px 0; line-height: 1.7; }
.notice { padding: 10px; background: #fff6e5; color: #76511d; font-size: 13px; border-radius: 6px; }
.messages { max-height: 280px; overflow: auto; }
.expand-button { padding: 6px 10px; background: white; border: 1px solid #ccd5e0; border-radius: 6px; cursor: pointer; }
.agent-panel.expanded { position: fixed; top: 10dvh; left: 4vw; right: 4vw; width: auto; max-width: 1100px; height: 80dvh; margin: 0 auto; box-sizing: border-box; z-index: 1100; display: flex; flex-direction: column; overflow: auto; box-shadow: 0 8px 48px #17324d33; }
.expanded header { flex-shrink: 0; }
.expanded .messages { flex: 1 1 auto; min-height: 100px; max-height: none; }
.expanded .composer { flex-shrink: 0; margin-top: 10px; }
.expanded .proposal { flex-shrink: 0; max-height: 24dvh; overflow: auto; }
.chat-message { background: #f4f7fc; border-radius: 8px; padding: 10px; margin: 10px 0; }
.chat-message p { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.7; margin: 6px 0; }
.proposal { margin: 16px 0; padding: 14px; border: 1px solid #b7ccec; border-radius: 8px; }
.proposal ul { padding-left: 18px; font-size: 14px; line-height: 1.8; }
.proposal-actions { display: flex; flex-wrap: wrap; gap: 8px; }.proposal-actions .el-button { margin: 0; }
.composer { display: grid; gap: 10px; margin-top: 18px; }.composer label { font-size: 13px; }
textarea { width: 100%; min-height: 64px; max-height: min(140px, 18dvh); overflow-y: auto; box-sizing: border-box; border: 1px solid #ccd5e0; border-radius: 6px; padding: 10px; resize: none; font: inherit; }
[role=alert] { color: #a83737; font-size: 13px; }
@media (max-width: 1000px) { .agent-panel { position: static; } }
@media (max-width: 600px) { .agent-panel.expanded { inset: 8px; height: calc(100dvh - 16px); padding: 12px; } .expanded header { position: sticky; top: 0; background: white; z-index: 1; } }
</style>
