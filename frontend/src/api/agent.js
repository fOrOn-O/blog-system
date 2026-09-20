import api from './index'
import { workspaceRequest } from '@/utils/knowledge'

export function chatWithWorkspace(message, workspace, mode = 'question') {
  return api.post('/chat', workspaceRequest(message, workspace, mode),
    { baseURL: import.meta.env.VITE_AGENT_API_BASE_URL || '/agent-api/api/v1/agent', timeout: 120000 })
}
