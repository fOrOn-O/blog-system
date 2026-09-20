import { ref } from 'vue'
import api from './index'

const config = () => ({ baseURL: import.meta.env.VITE_AGENT_API_BASE_URL || '/agent-api/api/v1/agent', timeout: 120000 })
export const askKnowledge = query => api.post('/knowledge/chat', { query }, config())
export const syncKnowledge = articleId => api.post(`/knowledge/sync/${articleId}`, null, config())

export const knowledgeSync = ref({})
const queued = new Set()
export function queueKnowledgeSync(articleId) {
  articleId = Number(articleId)
  if (!Number.isSafeInteger(articleId) || articleId <= 0) return
  if (knowledgeSync.value[articleId] === 'pending') { queued.add(articleId); return }
  knowledgeSync.value[articleId] = 'pending'
  Promise.resolve().then(() => syncKnowledge(articleId)).then(() => { delete knowledgeSync.value[articleId] })
    .catch(() => { knowledgeSync.value[articleId] = 'failed' })
    .finally(() => {
      // 同步期间发生新的发布事件时，完成后再按 Go 最新状态同步一次。
      if (queued.delete(articleId)) queueKnowledgeSync(articleId)
    })
}
