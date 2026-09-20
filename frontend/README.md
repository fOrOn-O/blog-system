# Vue 3 + Vite

This template should help get you started developing with Vue 3 in Vite. The template uses Vue 3 `<script setup>` SFCs, check out the [script setup docs](https://v3.vuejs.org/api/sfc-script-setup.html#sfc-script-setup) to learn more.

Learn more about IDE Support for Vue in the [Vue Docs Scaling up Guide](https://vuejs.org/guide/scaling-up/tooling.html#ide-support).
# Task 13：编辑工作区与 Agent

文章编辑页包含同页 Agent 面板、活动版本选择和共用结构化 Diff。普通问答及提案都调用
Task 11 的 `run_with_response()`；浏览器仅在内存保存展示消息，没有跨请求记忆。

新建文章先「保存草稿」获得真实 ID/V1。编辑已有文章可保存草稿、明确发布工作版本或归档；
原先立即发布的保存操作保留并标为「保存并发布」。历史版本只读，助手仍绑定所选历史版本。
活动工作区始终显示文章 ID/版本；后续聊天发送该工作区，但已有提案身份不会随切换而改变。
本地未保存内容不会发给 Agent，未保存编辑阻止 Apply，避免刷新覆盖本地输入。
图片上传、手动保存、工作区切换与 Apply 互斥；切换或刷新工作区后须重新预览原提案。
刷新成功后统一更新文章、版本列表、编辑器和工作区；刷新失败会显示错误并禁用编辑与 Apply，须重新加载。

提案状态：生成 → 预览中 → 已预览 → 用户确认 → 应用中 → 清除并刷新。
没有成功预览、工作区不匹配、有本地未保存编辑、提案已过期或请求执行中时，不能应用。
确认弹窗明确说明生成新工作版本且不发布。Apply 直接请求 Go，不发送「应用」消息给模型。
成功后读取文章与版本列表，载入响应中的新版本，公开指针取 Go 返回值，不推测发布状态。
409 显示过期/归档状态、保留原提案基础版本，提供刷新及丢弃；不自动重试、重绑、合并或变基。
其他 Apply 失败也不会自动重试，先刷新核实；刷新失败时不把已提交操作伪装成未执行。
丢弃只清本地提案、Diff、审核和错误状态，不存在 Reject API。

## 本地运行与部署接线

启动现有 Go Backend（默认 8080），在 agent-service 安装依赖并配置其 `.env` 后运行：

```sh
python -m uvicorn app.main:app --host 127.0.0.1 --port 8000
```

前端 `npm run dev` 默认将 `/api` 代理到 Go，将 `/agent-api` 去前缀后代理到 Python。
可用 `BLOG_PROXY_TARGET`、`AGENT_PROXY_TARGET` 调整开发代理目标。
前端运行时仅通过已有认证拦截器转发用户 JWT，不接触 Groq/Qdrant 密钥。

生产环境需配置同源反向代理：`/api/v1/*` 到 Go，`/agent-api/*` 去掉该前缀后到 Python。
Vite 开发代理不随静态构建部署。`VITE_API_BASE_URL` 和 `VITE_AGENT_API_BASE_URL`
可在构建时指定同源代理路径；若改为跨域部署，需另外明确配置服务端 CORS。
Agent 请求等待完整 JSON 响应，当前不引入 SSE、WebSocket 或持久化对话。

## HTML 与 Diff 边界

聊天文字、提案摘要、Diff 的 before/after 全部用 Vue 文本插值展示；不使用 `v-html` 渲染提案。
新 `ArticleDiff` 由历史版本比较与提案预览共用。此前前端没有历史版本/Diff 组件。
预览沿用 Task 08 的结构化语义，格式/HTML 属性变化可能无结构差异但仍生成新版本，UI 明确提示。
普通文章展示及编辑器载入内容复用既有 DOMPurify HTML profile，并不构造 Agent 专用 sanitizer。
Go 仍只有基础正文写入校验；浏览器渲染清洗不等同于服务端已清洗存储内容，其他消费端仍需安全处理。

## 自动化测试

```sh
npm ci
npm test
npm run build
npx playwright install chromium
npm run test:e2e
```

E2E 需要 Go（含现有 SQLite 驱动所需环境）和已安装 agent-service 依赖的 Python。
可设置 `AGENT_TEST_PYTHON` 为该解释器路径，`PLAYWRIGHT_CHROMIUM_EXECUTABLE` 为已有 Chromium 路径。
Playwright 自动启动并关闭 127.0.0.1 上的 18080/18000/15173 三个测试服务；端口占用会报错，不复用未知服务。
Go 使用测试内存 SQLite 和测试账号，不读取本地/生产数据库配置。Python 使用无密钥测试 Settings，
仅替换 LLM，保留真实 FastAPI、LangGraph、ToolNode、JWT、BlogClient、Go Handler/Service/Repository。
少数错误/恶意 HTML 场景在浏览器拦截响应；核心批准流程和 409 使用真实 Go 数据库操作。
测试不要求 Docker、Groq、E5 或 Qdrant。结果写入仓库忽略的 `tmp/task13-playwright`，不保存 JWT trace。

延后实测清单：Real Groq writing、Real E5、Real Qdrant、Real semantic retrieval、Real TiDB concurrent Apply。

## Task 13.5：站内知识与版本问答

新增登录后可访问的 `/knowledge`，面向全站当前已发布文章，不是“我的知识库”。
问题通过现有 Agent base URL 请求 `/knowledge/chat`，仅发送 query 和认证拦截器注入的 Bearer JWT。
Python 入口按 Go 验证后的用户 ID 进行服务端限流，默认 Agent Chat / Knowledge / HTTP 同步共用 10 次/分钟；
超限返回 429，前端只负责显示错误，不自动重试。health 不受影响。
页面提供 loading、错误、无依据、纯文本答案和结构化来源；来源链接由数值 article_id 生成，
标题/heading/答案不使用 v-html。每次提问独立，不发送聊天历史。

ArticleEdit 助手默认“版本问答”，由 Python 强制先检索绑定的已保存版本；
选择“写作提案”后才进入原有全文编辑提案流程，Preview/Apply 状态机不变。
请求只投影 message/mode/workspace，不提交未保存 HTML 或用户自填身份。

发布类操作、归档及删除在 Go 成功后独立发起 `/knowledge/sync/:id`；
同步失败不拒绝已成功的业务请求，导航栏下显示重试入口。关闭页面可能中断同步，
外部直接调用 Go 也不会经过前端 hook，需显式 Python reconcile；不声称同步是可靠消息投递。
草稿保存、提案生成、Preview、Apply 不触发知识同步。

生产构建仍只需原有 Go/Agent base URL 和代理，不能将 Qdrant/Groq 密钥写入 VITE 环境变量。
先 Review，再部署 Go 新 source API、Python Knowledge API 和前端；production full E2E 尚须单独验证。
本地浏览器 E2E 新增真实链路的跨文章来源、公开版本切换、归档过滤、exact 问答和同步失败隔离；
RAG 使用 FakeEmbedding + 内存 Qdrant，不依赖真实模型或云服务。
