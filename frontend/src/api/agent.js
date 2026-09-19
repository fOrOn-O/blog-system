import api from './index'

export function chatWithWorkspace(message, workspace) {
  return api.post('/chat', {
    message,
    workspace: { article_id: workspace.article_id, version_no: workspace.version_no }
  }, { baseURL: import.meta.env.VITE_AGENT_API_BASE_URL || '/agent-api/api/v1/agent', timeout: 120000 })
}
