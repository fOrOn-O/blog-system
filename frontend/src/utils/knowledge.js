export function workspaceRequest(message, workspace, mode = 'question') {
  if (!['question', 'write'].includes(mode)) throw new Error('Invalid workspace mode')
  return { message, mode, workspace: { article_id: workspace.article_id, version_no: workspace.version_no } }
}

export function sourcePath(source) {
  if (!Number.isSafeInteger(source.article_id) || source.article_id <= 0) return null
  return `/article/${source.article_id}`
}

// 发布成功即返回；派生索引失败只通知调用方，不改变业务结果或自动重试。
export async function commitThenSync(operation, schedule, articleId) {
  const result = await operation()
  try { schedule(articleId || result.data.id) } catch { /* 调度失败不改变已提交的业务结果。 */ }
  return result
}
