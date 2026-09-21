import test from 'node:test'
import assert from 'node:assert/strict'
import { assistantErrorMessage } from './assistant-error.js'

test('助手错误按认证、权限、限流、校验、超时和上游失败区分且不展示原始异常', () => {
  const cases = [[401, /登录已失效/], [403, /没有权限/], [429, /12 秒后/], [422, /请求无法处理/],
    [400, /请求无法处理/], [500, /助手服务暂时失败/], [502, /助手服务暂时失败/], [504, /响应超时/]]
  for (const [status, match] of cases) {
    const result = assistantErrorMessage({ response: { status, headers: { 'retry-after': '12' }, data: { message: 'SECRET stack trace', detail: 'SECRET' } } })
    assert.match(result, match)
    assert.ok(!result.includes('SECRET'))
  }
  assert.match(assistantErrorMessage({ response: { status: 429, headers: { 'retry-after': '<script>SECRET</script>' } } }), /稍后重试/)
  assert.match(assistantErrorMessage({ response: { status: 502, data: { detail: { code: 'model_rate_limited' } } } }), /模型服务繁忙/)
  assert.match(assistantErrorMessage({ response: { status: 502, data: { detail: { code: 'proposal_not_created' } } } }), /完整提案/)
  assert.match(assistantErrorMessage({ code: 'ECONNABORTED' }), /超时/)
  assert.match(assistantErrorMessage({ request: {} }), /检查网络/)
})
