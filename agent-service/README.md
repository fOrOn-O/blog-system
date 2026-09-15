# Blog Agent Service

FastAPI 服务提供进程健康检查、集中配置、异步 BlogClient 和最小 LangGraph Single Agent。
BlogClient 通过 HTTP 调用 Go Agent API，由 Go 执行认证授权、业务规则和数据库事务。
本服务不连接博客数据库，也不依赖 Go Backend 在线才能启动。

## 安装

要求 Python 3.12 或更高稳定版本，并已安装 [uv](https://docs.astral.sh/uv/getting-started/installation/)。
选择 3.12 作为下限，以使用成熟的 Python 版本并保留后续升级空间。

当前本机指定解释器为 `D:\Anaconda\python.exe`（Python 3.12.7）。
在仓库根目录进入服务目录，再按锁文件安装到该解释器：

```powershell
cd agent-service
uv export --locked --format requirements-txt --no-hashes |
    uv pip install --python 'D:\Anaconda\python.exe' --requirements -
```

使用安装而非同步清理，保留 Conda 环境中的无关包。其他环境可替换解释器路径，
或使用 `uv sync --locked` 创建项目 `.venv`。
FastAPI、Uvicorn、pydantic-settings、Pydantic 和 httpx 是运行依赖；pytest 用于测试。
Agent 使用 langgraph、langchain-core 和 langchain-groq。
Hatchling 仅用于构建 Python 包。依赖版本记录在 `uv.lock` 中。
锁文件中的 packaging 和 tenacity 保持与本机已有 Streamlit 的版本约束兼容。

## 配置与启动

以下命令均在 `agent-service/` 下执行。
默认配置即可启动；需要本地配置时，可复制 `.env.example` 为 `.env`：

```powershell
# PowerShell
Copy-Item .env.example .env
```

```bash
# Linux / macOS
cp .env.example .env
```

| 环境变量 | 默认值 | 用途 |
| --- | --- | --- |
| `APP_NAME` | `blog-agent` | 应用标题及健康响应中的服务名称 |
| `APP_ENV` | `development` | 运行环境名称 |
| `LOG_LEVEL` | `INFO` | 标准 logging 级别：DEBUG、INFO、WARNING、ERROR、CRITICAL |
| `BLOG_BACKEND_URL` | `http://localhost:8080` | Go 服务 origin，不含 API 路径、账号密码或查询参数 |
| `BLOG_BACKEND_TIMEOUT_SECONDS` | `10` | HTTP 连接、读取、写入和连接池等待的超时秒数，必须为有限正数 |
| `GROQ_API_KEY` | 空 | 仅运行真实 Groq 时需要，以 SecretStr 保存，不输出配置原文 |
| `LLM_MODEL` | `openai/gpt-oss-20b` | 唯一模型 Provider 为 Groq |
| `LLM_TIMEOUT_SECONDS` | `30` | 模型请求超时秒数 |
| `AGENT_RECURSION_LIMIT` | `12` | 单次运行的 Graph super-step 上限，配置范围 2～50 |

环境变量优先于当前目录的 `.env`，未设置时使用默认值。
配置加载后会缓存，修改后需要重启进程；不输出完整配置或环境变量。
`.env` 已被 Git 忽略，示例配置不包含密钥。

```powershell
& 'D:\Anaconda\python.exe' -m uvicorn app.main:app --host 127.0.0.1 --port 8000
```

浏览器访问 `http://127.0.0.1:8000/health`，默认返回 HTTP 200：

```json
{"status":"ok","service":"blog-agent"}
```

`/health` 只反映本进程能处理请求，不检查 Go、MySQL、Redis、向量数据库或模型服务。
API 文档位于 `http://127.0.0.1:8000/docs`。

## BlogClient

调用方为每次业务调用提供当前用户的原始 JWT（不含 `Bearer ` 前缀）。
Client 不解析 JWT 权限、不接受授权用的 `user_id`，也不保存全局用户 Token。
目前未新增 FastAPI 代理或聊天路由；未来入口负责从当前请求取得 JWT。

```python
from app.clients.blog import BlogClient
from app.clients.models import UpdateDraftInput


async def save_working_content(
    article_id: int, content: str, expected_version: int, access_token: str
):
    async with BlogClient() as client:
        return await client.update_draft(
            article_id,
            UpdateDraftInput(expected_version=expected_version, content=content),
            access_token=access_token,
        )
```

`expected_version` 必须是调用方开始编辑时读到的工作版本。
版本冲突交给调用方处理，Client 不重新获取或替换版本，也不自动重试。
一个实例可复用连接处理多个调用（包括不同用户），每次请求单独构造认证 Header。
使用 `async with` 管理生命周期，或由创建实例的调用方在结束时执行 `await client.aclose()`。
Client 不自动跟随重定向、不重试请求、不记录 Header 或 Token。

| 异步方法 | 返回类型 | Go API（前缀 `/api/v1/agent`） |
| --- | --- | --- |
| `get_article(article_id, *, access_token)` | `Article` | `GET /articles/:id` |
| `list_my_articles(*, access_token, page=1, limit=10, status=None)` | `ArticlePage` | `GET /articles` |
| `create_draft(draft: CreateDraftInput, *, access_token)` | `Article` | `POST /articles` |
| `update_draft(article_id, draft: UpdateDraftInput, *, access_token)` | `Article` | `PUT /articles/:id/draft` |
| `publish_article(article_id, *, expected_version, access_token)` | `Article` | `POST /articles/:id/publish` |
| `archive_article(article_id, *, access_token)` | `Article` | `POST /articles/:id/archive` |

`Article` 保留工作版本、发布版本、HTML 内容、标签、作者及现有计数和时间字段。
空标签被 Go 省略时解析为 `[]`；`ArticlePage.data` 是文章列表，`meta` 保留 Go 的分页信息。
输入模型拒绝未知字段。更新时未传入或为 `None` 的字段不发送；空字符串和空标签列表仍发送，
用于明确清空摘要、封面或标签。更新与发布必须提供正整数 `expected_version`。

| 失败 | 异常（均继承 `BlogClientError`） |
| --- | --- |
| 401 | `AuthenticationError` |
| 403 | `AuthorizationError` |
| 404 | `ArticleNotFoundError` |
| 409 | `VersionConflictError` |
| 其他 4xx | `BlogRequestError` |
| 5xx、重定向、响应格式不符合契约 | `BlogBackendError` |
| 超时、连接失败等 HTTP 传输错误 | `BlogBackendUnavailableError` |

异常保留 `method`、`path`、`status_code`。4xx 消息保留 Go 的业务说明并脱敏本次 JWT；
5xx 和格式错误不暴露原始响应，传输错误不携带原始 httpx 请求或异常消息。
本地输入错误由 Pydantic 或 `ValueError` 报告，在发送 HTTP 前拒绝。

当前 Go 的版本冲突、归档状态冲突和缺失快照均使用 HTTP 409，尚无独立机器可读业务码。
因此 `VersionConflictError` 表示 Go 的 409 冲突，具体原因仍需查看安全业务消息。
写请求发生网络错误时，不能据此断言 Go 没有执行；本 Client 不会自动补发。

## 测试

```powershell
& 'D:\Anaconda\python.exe' -m pytest
```

测试使用 FastAPI TestClient，覆盖健康响应、应用配置、默认配置、`.env` 加载、
环境变量优先级和日志级别校验。BlogClient 测试使用 httpx.MockTransport，覆盖六个业务接口、
身份隔离、请求与响应契约、错误映射、无重试、连接关闭和令牌脱敏，不需要启动 Go 或数据库。

## Minimal Single Agent

显式控制流定义在 `app/agent/graph.py`，未使用 `create_agent`：

```text
START → call_model → 有 tool_calls → tools（ToolNode）→ call_model
                  └ 无 tool_calls → END
```

State 只有 `MessagesState.messages`。每次运行从新的 HumanMessage 开始，
没有跨运行的对话记忆、Checkpointer、HITL、RAG 或流式接口。
`AgentContext` 仅携带当前 run 的 JWT 和 BlogClient；ToolRuntime 在执行工具时注入它，
JWT 不写入消息、普通 State、Prompt 或模型工具参数，也不出现在 Context 的 repr 中。

模型绑定与 ToolNode 执行使用同一份四工具白名单：

- `get_article(article_id)`：返回工作内容、Version、PublishedVersion 和标签。
- `list_my_articles(page=1, limit=10, status=None)`：返回本人文章与分页。
- `create_draft(title, content, summary='', cover_image='', tag_ids=None)`：只创建草稿。
- `update_draft(article_id, expected_version, title=None, content=None, summary=None, cover_image=None, tag_ids=None)`：只更新工作版本。

`publish_article` 和 `archive_article` 虽然存在于 BlogClient，但未绑定给模型，
也未注册到 ToolNode；模型即使编造对应调用也无法执行。
所有工具只调用 BlogClient，Go 仍负责最终权限、状态和事务规则。

成功结果为 `{"ok": true, "data": ...}`，只选择文章内容和版本等必要字段，
不向模型发送作者账号详情、计数或 HTTP 对象。
错误结果统一为 `{"ok": false, "error": "...", "message": "..."}`，
使用固定安全消息，不把异常原文或堆栈交给模型。
错误码包括 `authentication_required`、`permission_denied`、`article_not_found`、
`version_conflict`、`backend_unavailable`、`backend_error`、`invalid_request`、
`invalid_arguments` 和兜底的 `tool_error`。

版本冲突不会触发应用级重试或自动读取新版本覆盖。网络异常不能断言写入没有完成。
模型本身仍可能再次建议调用；当前使用短 Prompt 和 Graph 步数上限限制，
尚不提供请求去重或幂等保证。

`AgentRunner` 初始化时在 `app/core/model.py` 创建一次 Groq 模型并绑定工具，
模型节点只执行异步调用，不重复初始化。`ChatGroq` 将 `max_retries=0` 传给 Groq SDK，
表示模型 API 请求失败后不自动重试。该设置只影响模型请求，不控制 Tool 执行；
Graph 和 BlogClient 没有自动重试写 Tool 的策略，模型再次建议调用也不等于 SDK 重试。
Groq API 使用显式 Key，不做 Provider fallback。
Key 缺失时创建真实 Runner 会报配置错误，FastAPI `/health` 和假模型测试仍可运行。

调用方可以复用 Runner，并为每次调用提供当前用户 JWT：

```python
from app.agent.runner import AgentRunner

runner = AgentRunner()  # 需要配置 GROQ_API_KEY；只创建一次模型和 Graph


async def handle_message(message: str, access_token: str):
    answer = await runner.run(message, access_token=access_token)
    return answer.content
```

Runner 默认为本次 run 创建并关闭 BlogClient。也可通过 `blog_client=` 传入已有客户端，
此时连接生命周期由调用方负责。没有新增 `/chat` 或其他 Agent HTTP 路由。
每个模型节点和工具节点各消耗一个 super-step；默认 12 步对应最多约 6 轮模型/工具调用。
达到上限时抛出安全的 `AgentExecutionError`，不会无限循环；此前已完成的草稿操作不回滚。

Agent 自动测试使用 ScriptedModel 和 httpx.MockTransport，不调用真实 Groq，不需要 Key、
Go 服务或互联网，不消耗模型 Token。覆盖无工具回答、完整工具回路、多轮、并发身份隔离、
白名单限制、非法工具/参数、版本冲突、后端异常和步数上限。
LangGraph 本身会依赖 checkpoint 和 LangSmith 软件包，本项目未启用持久化或追踪服务。
真实 Groq 及真实 Go 全链路验证需在具备 Key、JWT 和 Go 服务后单独进行；不能以假模型测试代替。
