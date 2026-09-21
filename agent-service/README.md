# Blog Agent Service

FastAPI 服务提供进程健康检查、集中配置、异步 BlogClient、LangGraph Single Agent、版本内 Dense RAG 和文章编辑提案。
BlogClient 通过 HTTP 调用 Go Agent API，由 Go 执行认证授权、业务规则和数据库事务。
本服务不连接博客数据库，也不依赖 Go Backend 在线才能启动。

## 安装

要求 Python 3.12 或更高稳定版本，并已安装 [uv](https://docs.astral.sh/uv/getting-started/installation/)。
选择 3.12 作为下限，以使用成熟的 Python 版本并保留后续升级空间。

当前本机指定解释器为 `D:\Anaconda\python.exe`（Python 3.12.7）。
在仓库根目录进入服务目录，再按锁文件安装到该解释器：

```powershell
cd agent-service
uv export --locked --extra local --format requirements-txt --no-hashes |
    uv pip install --python 'D:\Anaconda\python.exe' --requirements -
```

使用安装而非同步清理，保留 Conda 环境中的无关包。其他环境可替换解释器路径，
或使用 `uv sync --locked --extra local` 创建项目 `.venv`。
FastAPI、Uvicorn、pydantic-settings、Pydantic 和 httpx 是运行依赖；pytest 用于测试。
Agent 使用 langgraph、langchain-core 和 langchain-groq。
Qdrant Client 是运行依赖；sentence-transformers 属于可选的 `local` extra，
只在本地 E5 环境安装。Qdrant Cloud Inference / Render 环境不要安装 `--extra local`，不安装 PyTorch 和模型运行库。
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
| `get_version_diff(article_id, from_version, to_version, *, access_token)` | `ArticleVersionDiff` | `GET /articles/:id/diff?from_version=...&to_version=...` |
| `get_version_chunks(article_id, version_no, *, access_token)` | `ArticleVersionChunks` | `GET /articles/:id/versions/:version/chunks` |
| `get_article_version(article_id, version_no, *, access_token)` | `ArticleVersion` | `GET /articles/:id/versions/:version` |
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
没有跨运行的对话记忆、Checkpointer、HITL 或流式接口。
`AgentContext` 携带当前 run 的 JWT、BlogClient、可选 RAG 服务，以及 Task 11 的可选工作区和运行内提案容器；ToolRuntime 在执行工具时注入它，
JWT 不写入消息、普通 State、Prompt 或模型工具参数，也不出现在 Context 的 repr 中。

旧 `run()` 入口的模型绑定与 ToolNode 执行使用同一份六工具白名单：

- `get_article(article_id)`：返回工作内容、Version、PublishedVersion 和标签。
- `list_my_articles(page=1, limit=10, status=None)`：返回本人文章与分页。
- `get_version_diff(article_id, from_version, to_version)`：只读比较本人文章的两个历史版本，要求 `0 < from_version < to_version`，支持非相邻版本。
- `search_article_version(article_id, version_no, query)`：只读检索本人文章的指定历史版本，不自动建立索引。
- `create_draft(title, content, summary='', cover_image='', tag_ids=None)`：只创建草稿。
- `update_draft(article_id, expected_version, title=None, content=None, summary=None, cover_image=None, tag_ids=None)`：只更新工作版本。

`publish_article` 和 `archive_article` 虽然存在于 BlogClient，但未绑定给模型，
也未注册到 ToolNode；模型即使编造对应调用也无法执行。
文章工具通过 BlogClient 访问 Go；检索工具调用 RAG 服务，并先通过 BlogClient 由 Go 校验权限及版本。
Go 仍负责最终权限、状态和事务规则。

成功结果为 `{"ok": true, "data": ...}`，只选择文章内容和版本等必要字段，
不向模型发送作者账号详情、计数或 HTTP 对象。
错误结果统一为 `{"ok": false, "error": "...", "message": "..."}`，
使用固定安全消息，不把异常原文或堆栈交给模型。
错误码包括 `authentication_required`、`permission_denied`、`article_not_found`、
`version_conflict`、`backend_unavailable`、`backend_error`、`invalid_request`、
`invalid_arguments`、`rag_unavailable` 和兜底的 `tool_error`。

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

## 确定性版本 Diff

Diff 由 Go Service 根据不可变快照计算，不调用 LLM。Python 只通过 BlogClient 调用
受 JWT 和所有权保护的 Go Agent API；工具参数不包含用户身份，JWT 由 ToolRuntime 注入。
不需要启动模型服务即可直接调用 `BlogClient.get_version_diff(...)`。

`ArticleVersionDiff` 返回 `article_id`、`from_version`、`to_version`、`field_changes` 和 `content`。
`field_changes` 固定包含 title、summary、cover_image 的 `{changed, before, after}`；
`content` 包含 `{changed, changes}`，其中每项记录块的 `operation`、`before_index`、
`after_index`、`before`、`after`。块包含 `{type, text}`，索引从 0 开始；缺失的一侧为 null。
没有变化时 `changes` 为 `[]`。状态、标签、PublishedVersion、计数及作者信息不参与比较。

HTML 先归一化成 heading、paragraph、list_item、blockquote、code_block，再比较有序块。
本阶段忽略行内格式、属性、标题级别和列表/引用嵌套深度；普通文本空白合并，代码块保留缩进和换行。
因此 `<p>Redis</p>` 与 `<p><strong>Redis</strong></p>` 可视为相同内容。
正文图片、嵌入媒体及表格结构不在本阶段语义比较范围内；这不是完整富文本 Diff，
`content.changed=false` 只表示归一化后的语义块相同，不表示原始 HTML 完全相同。
Diff 文本是数据，展示方应按纯文本转义，不将其作为 HTML 执行。

400、401、403、404 和 5xx 复用既有 BlogClient 错误映射。
缺失历史版本使用 404，因此仍映射为 `ArticleNotFoundError` 和工具的 `article_not_found`；
既有 409 映射保持不变，Diff 本身不执行工作版本冲突检查，不更新任何业务数据。
完整 Go API 响应示例与算法说明见 [后端 README](../backend/README.md)。

## Task 10 / 13.5-A：指定文章版本的 Dense RAG

```text
显式 index → BlogClient → Go JWT / 所有权校验 → ArticleVersion → Task 09 ChunkHTML / RenderText
          → RetrievalBackend.replace → 本地 E5 或 Qdrant Cloud Inference → Qdrant

用户问题 → 现有 LangGraph → search_article_version → Go 再次校验所有权及版本
         → RetrievalBackend.search → Qdrant 候选集 → 校对 Go 原文 → 去重 → 最多 Top-K
         → RetrievedChunk → 按检索顺序组装上下文 → Tool Result → 同一个模型生成答案
```

Python 不解析 HTML，不访问博客 SQL 数据库，不改变文章、版本或发布状态。
`get_version_chunks` 返回的 `user_id` 由 Go 从验证后的 JWT 取得，Python 不自行解析 JWT 或接收模型指定的用户 ID。
每次检索都重新请求 Go 的该版本分块，确认文章未删除、版本存在、用户仍有所有权；
Qdrant 命中的文本和 HeadingPath 还会与 Go 响应逐项比较，过期或不匹配内容被舍弃。

### 本地配置与启动

集中配置在 `app/core/config.py`；客户端和 provider 工厂在 `app/core/rag.py`。
环境变量覆盖 `.env`，缺省值对应本地开发。切换 profile 或连接配置后必须重启进程。

| 配置 | 本地默认值 | 含义 |
| --- | --- | --- |
| `APP_ENV` | `development` | 本地开发；Render 应显式设置 `production` |
| `EMBEDDING_PROVIDER` | `local_e5` | 选择 provider 工厂分支，不是根据 URL 猜测 |
| `EMBEDDING_MODEL` | `intfloat/multilingual-e5-small` | E5 多语言模型 |
| `EMBEDDING_DIMENSION` | `384` | 本地 E5 固定向量维度 |
| `QDRANT_DISTANCE` | `Cosine` | 本地 E5 固定距离 |
| `QDRANT_COLLECTION` | `article_chunks_e5_v1` | 整个 embedding profile 共用的集合，不按用户或文章建集合 |
| `QDRANT_URL` | `http://localhost:6333` | 可改为外部持久化 Qdrant / Qdrant Cloud 的 HTTPS origin |
| `QDRANT_API_KEY` | 空 | 外部 Qdrant 密钥，SecretStr；有密钥时要求 HTTPS |
| `QDRANT_TIMEOUT_SECONDS` | `10` | Qdrant 请求超时秒数 |
| `EMBEDDING_DEVICE` | `cpu` | 本地推理设备 |
| `EMBEDDING_BATCH_SIZE` | `32` | 本地文档 embedding batch size |
| `RAG_TOP_K` | `5` | 1～50，由应用配置决定，不暴露给模型 |

本地 Docker Qdrant（需要先安装并启动 Docker Desktop）：

```powershell
docker compose up -d qdrant
```

`compose.yaml` 使用 Qdrant 1.16.3，仅将 HTTP 端口绑定到 `127.0.0.1:6333`，
索引存放在 Docker named volume `qdrant_data`；`docker compose down` 保留数据，`down -v` 会删除索引卷。
外部 Qdrant 至少需要 1.16，因为集合 profile 使用该版本开始支持的
[collection metadata](https://qdrant.tech/documentation/manage-data/collections/)。
修改 `QDRANT_URL` 和密钥即可使用外部 Qdrant，无需修改索引、检索代码或挂载 Render 本地磁盘。

安装前述 `local` extra 后，可选预下载并预热模型：

```powershell
& 'D:\Anaconda\python.exe' -m app.rag warm-model
```

这会通过 sentence-transformers 下载到正常的 Hugging Face 本地缓存，默认不使用仓库目录；
需联网且预留模型缓存空间。预热进程退出后，服务仍需首次将缓存权重加载到内存，随后复用同一实例。
`LocalE5EmbeddingProvider` 在第一次真实推理时延迟加载，加载和推理有锁保护；
工厂在同一进程复用 provider 和 Qdrant client，不会每次请求重新加载模型。
FastAPI `/health` 和普通单元测试均不下载或加载真实模型；CLI 和 FastAPI lifespan 会关闭已创建的 Qdrant client。
模型权重、缓存和 `.env` 不提交到 Git。

先启动本地 Go Backend，使用真实登录取得本人 JWT，并从 Go API 确认文章和版本存在。
以下命令的 ID/版本仅为示例，替换为自己的数据；命令运行时隐藏输入 JWT，不把 Token 写在命令行或文档中：

```powershell
& 'D:\Anaconda\python.exe' -m app.rag index --article-id 18 --version-no 1
& 'D:\Anaconda\python.exe' -m app.rag ask --article-id 18 --version-no 1 --question '这个版本如何介绍 Redis 持久化？'
```

`index` 只需要 Go、embedding 和 Qdrant；`ask` 使用现有 `AgentRunner`，还需配置 `GROQ_API_KEY`。
没有增加聊天 HTTP API。应用内部也可调用 `get_rag_service().index_article_version(...)`；
检索工具通过 Runtime Context 获得本次 JWT，身份不会进入工具 schema、上下文文本或模型输入。
自定义 Runner 测试/配置应显式传入相应的 `rag_service`；不传时使用集中配置的进程级实例。

### Profile 隔离、点模型与重建

`RetrievalBackend` 协议位于 `app/rag/backend.py`，提供异步 `replace(source)`、
`search(source, query, limit)`、`aclose()`。ArticleRagService 只负责 Go 授权、canonical 分块校验与结果截取，
不关心向量在哪里计算。`LocalRetrievalBackend` 组合既有 EmbeddingProvider 与 QdrantChunkStore；
`QdrantCloudRetrievalBackend` 通过 Document 直接请求云端推理，不伪造返回 float[] 的远程 provider。
本地 `EmbeddingProvider` 协议仍位于 `app/rag/embedding.py`，提供 `dimension`、`profile`、
`embed_documents` 和 `embed_query`。
本地实现使用 `passage: <chunk text>` 和 `query: <question>`，并归一化向量；
前缀只存在于推理输入，原文、Go Chunk 和 Qdrant payload 都不会被添加前缀。

集合创建时写入 metadata：

```json
{"embedding_profile":{"provider":"local_e5","model":"intfloat/multilingual-e5-small","dimension":384,"distance":"Cosine","collection":"article_chunks_e5_v1"}}
```

每次读写都核对实际 size、distance 和完整 profile。即使维度相同，只要模型或 provider 不同就拒绝使用；
现有无 profile 的集合也不会被自动认领或删除。更换模型时必须使用新集合、重新显式索引。
本地 Provider 和 Store 在组装 backend 时也必须具有完全一致的 profile；未知 provider 明确报错，不回退到本地 E5。
`local_e5` 与 `qdrant_cloud` 即使模型、维度、距离相同，仍属于不同 profile，不能自动共用已有集合。

一个 Go Chunk 对应一个 Point。ID 为固定 UUID namespace 下
`blog-system/article/{article_id}/version/{version_no}/chunk/{chunk_index}` 的 UUIDv5。
payload 仅包含：`user_id`、`article_id`、`version_no`、`chunk_index`、`heading_path`、`text`；
前三个字段建立整数 payload index。没有 HTML DOM，也不保存完整 Blocks。
检索和旧点枚举均限定 `user_id AND article_id AND version_no`，不进行跨文章检索。
内部候选数量为 `min(RAG_TOP_K * 3, 100)`；按 Qdrant 返回的相似度顺序校验 chunk_index、text、heading_path，
跳过 stale/mismatched 内容并去重，最终最多返回 RAG_TOP_K。身份或 point ID 异常直接抛安全 RagError。
候选过取能缓解 stale 点占位，但有限候选集不保证过滤后一定凑满 Top-K。

本地重建顺序：获取所有 Go chunks → 生成所有向量 → 校验数量、维度、有限非零值 →
校验集合 profile → 删除该用户/文章/版本旧点 → 批量 upsert 新点。
因此旧 chunk 3 在新结果仅有 0～2 时会被清除，空结果会清空该版本索引，重复索引不会增加逻辑点数。
embedding 失败发生在删除前；删除成功而 upsert 失败时可能暂时缺少索引，显式重试可修复。
该过程不是数据库事务，不自动重试写操作；当前仅在单进程内串行化替换操作，未提供跨进程写入锁。

### Render / 生产配置边界

Task 13.5-A 增加正式 `qdrant_cloud` profile，客户端通过集中配置创建并缓存：
`AsyncQdrantClient(..., cloud_inference=True, check_compatibility=False)`。
本地 profile 显式使用 `cloud_inference=False`，保留本地 E5 + Docker Qdrant；生产使用 Cloud Inference + 外部持久化 Qdrant Cloud。

在 Render 配置以下值（URL 与 Key 仅示例，不要写入 Git）：

```dotenv
APP_ENV=production
EMBEDDING_PROVIDER=qdrant_cloud
EMBEDDING_MODEL=intfloat/multilingual-e5-small
EMBEDDING_DIMENSION=384
QDRANT_DISTANCE=Cosine
QDRANT_COLLECTION=article_chunks_e5_cloud_v1
QDRANT_URL=https://YOUR-CLUSTER.cloud.qdrant.io
QDRANT_API_KEY=<render secret>
```

Cloud profile 强制上述模型、384 维、Cosine，以及 HTTPS URL 和非空 Key。
仍需保持现有 Go `BLOG_BACKEND_URL`、`GROQ_API_KEY`、`LLM_MODEL` 配置。
本地也允许选择 qdrant_cloud 做后续真实 smoke test，不强制 APP_ENV=production。
`production` / `prod` 下的 local_e5 在工厂创建时拒绝；未知 provider 同样拒绝，绝不 fallback。

生产安装使用 `uv sync --locked --no-dev`，不要加 `--extra local`，不要安装 fastembed、sentence-transformers 或 PyTorch。
普通 runtime 依赖不变，qdrant-client 保持既有版本范围。Cloud profile 执行 `warm-model` 会明确拒绝，
它不拥有本地模型；`index` 和 `ask` CLI 的调用方式、隐藏 JWT 输入保持不变。

Cloud 写入使用 `models.Document(text=chunk.text, model=settings.embedding_model)`；
查询使用 `models.Document(text=query, model=settings.embedding_model)`，**不手动添加 passage/query 前缀**。
Cloud E5 推理路径负责所需前缀；原文 payload 与 Go 保持一致。
Document 的 SDK 用法见 [Qdrant Cloud 官方说明](https://qdrant.tech/documentation/cloud/quickstart-cloud/)。

Cloud 重建顺序：

```text
Go 校验并获取完整版本 chunks
→ 校验/创建 collection 及 profile、整数 payload indexes
→ 按三重身份过滤，完整 scroll 分页枚举 existing IDs
→ 计算 deterministic expected IDs
→ 分批 upsert Document（每批 wait=True）
→ 分批 retrieve 全部 expected IDs（不下载 vectors）
→ 检查每批数量、ID 集合及完整 payload 与 Go 一致
→ 精确删除 existing - expected 的 stale IDs（每批 wait=True）
```

任何 upsert/inference 或验证失败都不执行 stale delete，只抛安全 RagError，不返回 SDK、网络或密钥原文。
空 chunks 不调用推理，校验集合后仅清理该版本已枚举的点。查询不创建集合；只有显式 index 才创建缺失集合。
不兼容集合会明确报 RagConfigurationError，不删除、重建或自动认领。

这不是跨请求原子事务：前几批 upsert 可能已完成，失败不会回滚已写的点；旧 stale 点保留到后续显式 index 成功。
全部验证完成后若 stale delete 失败，也可能只删除了部分 stale 点，可再次显式 index 恢复。
替换仍只有单进程互斥，没有跨实例写入锁，不自动重试写请求。
检索始终先通过 Go 校验权限/版本，再按 canonical chunks 过滤，不能把未校验的旧点当作事实。

Publish 不自动 index；不增加跨版本、Published Knowledge Index 或新页面。
`/health` 只代表进程存活，不代表 Cloud 推理/检索就绪。
代码和普通测试完成不代表 production RAG 已上线；仍须配置 Render secrets、部署，
然后使用本人真实 JWT 和已确认的 ArticleVersion 显式 index/search 做 Cloud smoke test。

### 测试与当前限制

```powershell
& 'D:\Anaconda\python.exe' -m pytest
```

新增测试使用 FakeEmbedding、FakeModel、httpx.MockTransport 和 Qdrant SDK 内存模式；
覆盖前缀、单次加载、profile/实际集合规格校验、点身份、重建、失败保护、三重过滤、Top-K、
Go 原文校对及真实 LangGraph/ToolNode/JWT Context 回路。内存模式不实际建立 payload 索引，
测试另外校验建索引请求。Cloud 测试模拟分页/写入/验证/失败，并使用真实 SDK + Mock HTTP 校验 Document 序列化和禁用本地推理。
测试不下载模型、不调用真实 Cloud，也不能据此宣称已验证 Docker/Cloud 网络或真实模型检索质量。

当前只支持单个指定文章版本的 Dense Top-K。未索引时返回空结果，不自动建索引；
空结果不证明该版本没有相关内容，Agent 应说明没有可用检索依据。
无相似度阈值或 reranker；上下文保持 Qdrant 返回顺序，最终自然语言生成仍依赖模型。
Go Task 09 按字符/语义块分块，而 E5 超过模型的 512 token 上限时会截断 embedding 输入，
详见 [E5 模型说明](https://huggingface.co/intfloat/multilingual-e5-small)。原文仍完整保存；本任务不增加 token 重分块。
每次检索为保证 Go 是权限与内容真源会重新获取整份版本分块，当前以正确性优先，尚未优化该开销。

## Task 11：文章写作 / 编辑提案

Task 11 使用 `AgentRunner.run_with_response()`，返回 `AgentResponse(answer, proposal)`。
它支持无工作区的普通问答，也支持绑定现有历史版本的重写、扩写、精简、结论、语气和结构调整。
旧 `run()` 仍返回 `AIMessage`，保留 Task 01～10 的工具行为；**需要只生成提案时必须使用新入口**。
没有新增 FastAPI 聊天路由或前端功能。

```python
from app.agent.models import AgentWorkspace
from app.agent.runner import AgentRunner

runner = AgentRunner()


async def propose_edit(message: str, access_token: str, article_id: int, version_no: int):
    # ID 来自调用方选择的工作区，不从模型回答或工具参数中提取；最终权限仍由 Go 判断。
    result = await runner.run_with_response(
        message,
        access_token=access_token,
        workspace=AgentWorkspace(article_id=article_id, version_no=version_no),
    )
    return result.model_dump(mode="json")


async def chat(message: str, access_token: str):
    return (await runner.run_with_response(message, access_token=access_token)).model_dump(mode="json")
```

结构化返回示例：

```json
{
  "answer": "已生成编辑提案，尚未保存。",
  "proposal": {
    "article_id": 23,
    "base_version_no": 7,
    "proposed_content": "<h1>Redis</h1><p>完整编辑后的正文。</p><p>保留的其他段落。</p>",
    "change_summary": ["改写介绍段，保留其他段落"]
  }
}
```

普通问答的 `proposal` 为 `null`。提案模型没有持久化 ID、审批状态、时间戳或新版本号；
`base_version_no` 始终是工作区选择的历史版本，并不表示当前最新工作版本。

调用链：

```text
调用方的 AgentWorkspace + 当前 JWT → 每次运行独立的 AgentContext
→ read_workspace_article（无模型可见参数）
→ BlogClient.get_article_version → Go JWT / Owner 校验 → 指定 ArticleVersion
→ 原样完整 HTML 返回同一个 Agent LLM → 生成完整 proposed_content + change_summary
→ submit_article_edit_proposal → Go 再次校验该版本 → 校验提案
→ 当前运行的 ProposalCapture → AgentResponse.proposal
```

读取接口为 `GET /api/v1/agent/articles/:id/versions/:version`，返回快照的文章 ID、版本号、
Title、Content、Summary 和 CoverImage；不混入 Article 的当前工作内容、状态、标签或计数。
Python 不访问业务数据库，Qdrant、Task 10 的 RenderText 和 `get_article` 的当前工作内容都不能充当编辑基准。
提交前必须已有成功的 `read_workspace_article` 结果进入上一轮消息；同一轮并行读取/提交不能跳过该要求。
提交时再次读取 Go，若身份失效、权限不符或版本消失，按原有安全错误模型拒绝，提案不会被捕获。

`submit_article_edit_proposal` 的模型参数只有 `proposed_content` 和 `change_summary`。
文章/版本绑定来自冻结的 `AgentWorkspace`，JWT 来自 Runtime；模型额外传入的身份字段不能覆盖它们。
`ProposalCapture` 每次运行重新创建，只暂存该次读取结果和第一份成功提案，不解析 LLM 最终自然语言中的 JSON，
不跨请求保留记忆、不持久化。并发请求分别持有自己的工作区、JWT 和提案。
同一模型轮次包含多次提交时，在 Go 校验前全部拒绝，返回 `multiple_proposal_submissions`，不按网络完成顺序挑选。
第一份成功的单次提交后，图直接返回固定的待预览/确认说明，不再调用模型，也不给下一轮覆盖提案的机会。
工具自身仍保留 `proposal_already_submitted` 防御检查。

新入口的白名单固定为四个既有只读工具（get/list/diff/search）和上述两个工作区工具。
模型绑定和 ToolNode 均不注册 create_draft、update_draft、publish、archive 或索引操作，
即使模型要求执行这些名字，也无法通过新入口触发业务写入。RAG 仍使用 Task 10 原有版本内检索逻辑。
旧入口与新入口复用同一个模型实例和同一个 `build_graph` 构造函数，分别缓存对应白名单的图；
StateGraph 的节点、边和 MessagesState 不变，没有写作工具中的第二个 LLM。

提案校验使用标准库 HTMLParser：拒绝空正文、纯文本、Markdown 包裹、标签未闭合/不匹配、
空内容以及脚本/事件属性等明显活动内容，检查 change_summary 是非空字符串列表。
校验不改写 HTML，不生成 patch，不做 HTML 分块；要求显式闭合非 void 标签，符合编辑器通常输出的 HTML 片段形式。
它不能确定模型是否语义上遗漏了原文，也不是浏览器级完整 HTML/XSS sanitizer；全文保留由模型指令约束。
展示提案应按不可信内容处理。Task 12/13 的最终应用与安全流程仍需重新授权、检查基础版本、执行最终内容校验及必要的 HTML 安全处理，
然后由 Go 在事务中保存并生成版本；人类审批和这些应用流程本任务均未实现。

测试使用 Fake Model、MockTransport 和 Go SQLite 内存库，不需要真实 Groq、下载 embedding 权重或启动 Qdrant。
Python 测试验证提案链路仅发 GET，所有 BlogClient 写方法均未调用，并验证非法写工具不能执行；
Go 测试对照重复读取前后的 Article 与全部 ArticleVersion，确认内容、版本、PublishedVersion、标签、计数及缓存不变。

## Task 12：应用侧人工批准边界

Task 11 的 `AgentResponse.proposal` 仍为短期应用数据。应用可将其中的文章 ID 放入路由，
仅取 `base_version_no`、`proposed_content`，携带当前用户 JWT 直接调用 Go：

- `POST /api/v1/articles/:id/edit-proposal/preview`：使用基础历史快照和 Task 08 的结构化 diff 预览。
- `POST /api/v1/articles/:id/edit-proposal/apply`：用户明确批准后保存下一工作版本。

接口及错误语义见 [Go Task 12 说明](../backend/README.md#task-12人工批准与安全应用)。
本阶段没有 FastAPI 转发接口，也没有 BlogClient Apply 方法或 LangGraph Apply 工具。
`run_with_response()` 的 model.bind_tools 和 ToolNode 仍只注册 get/list/diff/search、
read_workspace_article、submit_article_edit_proposal，不能调用保存、应用、发布、归档或索引。
旧 `run()` 保持兼容，未来编辑 UI 应使用 `run_with_response()`。

应用时 Go 重新认证、检查所有权及当前版本，拒绝过期提案；不信任 Task 11 已通过的权限检查。
成功只生成未发布工作版本，PublishedVersion 保持不变，不触发 embedding/Qdrant。
没有提案持久化、批准状态或自动重试/合并；拒绝只需丢弃提案。
Go 当前的共同正文规则仅检查非空，未解决完整 HTML 安全清洗。
Task 13 的预览渲染和批准交互必须把 HTML 视为不可信内容。
# Task 13：工作区聊天 HTTP 入口

`POST /api/v1/agent/chat` 接受用户 Bearer JWT 和：

```json
{"message":"改写介绍段","mode":"write","workspace":{"article_id":23,"version_no":7}}
```

返回 `answer`、`proposal`，以及问答用的 `has_evidence` / `sources`（写作模式为 null / 空数组）。
请求不接受 user_id、token 或额外 workspace 身份字段。入口先通过 BlogClient 向 Go 验证
指定历史版本的所有权和存在性，再创建/复用 Runner，将工作区和请求 JWT 注入运行上下文。
Go 的 401/403/404 保持对应状态，模型/上游失败返回脱敏 502，模型超时返回 504；消息为空或结构非法返回 422。
Task 13.5 中 HTTP 默认 `mode=question`，强制检索活动版本后回答；`mode=write` 保留完整快照与提案图。
Runner/模型按进程延迟复用；每次请求仍是独立运行，无 checkpoint、对话持久化或记忆。

写作模式只能调用 `run_with_response()` 提案白名单。Preview/Apply 由前端直接请求 Task 12 Go 应用接口，
没有新增 Apply 工具或 Python 业务数据库写入。使用同源前端反向代理接入，部署说明见 frontend README。
浏览器 E2E 的 tests/e2e_app.py 仅用于测试：替换 LLM，配置独立本地 Go fixture，不加载 `.env` 密钥。

## Task 13.5：版本问答与全站已发布知识

两种检索范围使用独立 collection，不混用 active published view 和历史版本：

| 范围 | Cloud collection | 事实与权限 |
| --- | --- | --- |
| ArticleEdit 的 exact version | `article_chunks_e5_cloud_v1` | Go 校验当前 JWT 所有权及 article_id/version_no，仍需显式 index |
| 全站 Published Knowledge | `published_knowledge_e5_cloud_v1` | Go 决定当前公开快照，所有登录用户查询同一个全站范围，无 user_id/ownership 过滤 |

本地配置 `PUBLISHED_KNOWLEDGE_COLLECTION=published_knowledge_e5_v1`；Render 配置
`PUBLISHED_KNOWLEDGE_COLLECTION=published_knowledge_e5_cloud_v1`。它必须与 `QDRANT_COLLECTION` 不同。
两者分别写入包含自身 collection 名称的 embedding_profile metadata，模型同为 E5-small / 384 / Cosine。
本地继续复用单个 LocalE5EmbeddingProvider；Cloud 不安装/加载本地模型，不添加 passage/query 前缀。

### Go canonical source 与登录边界

只读 API：`GET /api/v1/agent/published-knowledge` 返回全站数组；
`GET /api/v1/agent/published-knowledge/:id` 返回单篇的零或一个记录。
记录包含 `article_id`、公开快照 `title`、`published_version`、Task 09 canonical `chunks`。
Go 以同一 SQL join 选择 status=published、published_version>0 且对应快照存在的未删除文章。
工作草稿、历史版本、归档、缺失快照不返回；单篇未公开与不存在统一为 `[]`，不增加浏览量。

当前 Python Knowledge API 和 Go source API 入口均要求 Bearer JWT。JWT 在 Python 中只作为 Go API 凭据，
不进入 payload、向量过滤条件、模型上下文或 source DTO；Published retrieval 不接收 user_id，也不校验 ownership。
未来开放匿名访问时调整 API/Go source 客户端的认证约定即可，无需修改检索算法或迁移集合。
Exact version 的 owner 权限不因 Published Knowledge 而放宽。

### 单进程请求限流

Python Agent Chat、Knowledge Chat 和 HTTP 单篇知识同步共用每个已认证用户的滑动窗口额度：
`CHAT_RATE_LIMIT_REQUESTS=10`、`CHAT_RATE_LIMIT_WINDOW_SECONDS=60`。
入口先调用 Go `/api/v1/user/profile` 获取经验证的用户 ID，再检查额度，超限不读取全文、不查询 Qdrant、不调用模型。
更换 JWT 不会绕过同一用户额度；ID 不传入 site-wide retrieval，不作为 ownership 过滤条件。
429 返回固定 `detail.code=rate_limit_exceeded`、中文安全消息及 `Retry-After` 秒数，不返回凭据或后端异常。
同步被限流不改变已提交的 Publish；可等待后通过页面手动重试，运维 reconcile 不通过普通 Agent tool 暴露。

限流状态只有进程内内存，使用锁保护并发请求，不记录 JWT、不持久化 key、不增加 Redis 或其他服务。
最多保留 10000 个活动主体，过期条目自动清理；进程重启清零。health 不受限。
当前 Render 应保持单实例、单 Python worker；多 worker 也会有独立计数，不能视为全局共享配额。
`Limiter` 协议和集中 `get_request_limiter()` 工厂允许以后替换分布式 backend；
主体命名空间为 `("user", verified_id)`，未来匿名入口可提供可信 `("ip", address)`，当前不信任客户端提交的 IP/subject。
此阶段不实现匿名访问、分布式限流或持久化限流。

### 同步与全站修复

`POST /api/v1/agent/knowledge/sync/:article_id` 只修派生索引，不调用任何 Go 写接口。
前端在 Go 创建并发布、保存并发布、发布、归档、删除成功后独立调度同步；业务请求立即返回，
不等待 embedding。失败只显示“业务已完成，索引同步失败”及重试入口；不回滚业务、不自动重试失败请求。
同一文章同步期间若发生新的业务操作，会在当前同步结束后再按最新 Go 状态同步。
保存草稿、Proposal、Preview、Apply 不触发 Published sync，也不自动建立 exact index。

这不是可靠消息投递：关闭页面、其他客户端直接调用 Go、跨实例竞态可能使索引暂时 stale。
应在首次部署、外部批量发布或同步失败后显式运行 reconcile；不添加队列、后台记忆或强事务。

```sh
cd agent-service
python -m app.knowledge sync --article-id 18
python -m app.knowledge reconcile
```

JWT 在终端隐藏输入，不作为命令行参数或持久化配置。reconcile 获取 Go 全站公开 ID 和索引内已存在 ID 的并集，
逐篇重新读取 Go 当前状态，以修复遗漏文章、旧版本、重复旧点、已归档/删除文章及半途失败。
实现使用全站源列表和 Qdrant scroll 分页；目前适合此个人博客规模，尚未分页传输 Go 全站正文。

替换沿用 13.5-A 的共同写入函数：准备全部点 → 分批 UPSERT(wait=True) → 验证全部 ID/数量/完整 payload
→ 精确 DELETE stale(wait=True)。旧版本 UUID 也在该文章 stale 范围内。
任何写入或验证失败都不提前删除旧点；已写批次不会事务回滚，显式 sync/reconcile 可恢复。
空/非公开目标不推理，清理该文章已有点。进程内锁包含源查询与替换，没有跨进程锁；
操作期间 Go 仍可更新，修复期间查询使用 canonical 校验，不把索引中的状态当作事实。
SDK 错误对外为固定安全消息，不打印密钥、JWT 或网络异常原文。

### 强制检索与结构化来源

`POST /api/v1/agent/knowledge/chat` 请求仅为 `{"query":"问题"}`，拒绝 user_id/article_id/version_no/top_k/conversation_id。
服务固定执行 `search_published_knowledge` → canonical 校验 → grounded answer，不让模型决定是否检索。
查询不带身份过滤，只查询 published collection；在查询后再次读取 Go，核对 article/version/index/title/text/heading_path。
内部取 `min(RAG_TOP_K*3,100)` 候选，按原始排名去重，每篇最多两块，最终最多 RAG_TOP_K，score 保持不变。
候选过取和每篇上限不保证一定命中多篇；当前没有 reranker、相关性阈值或检索质量评测。

返回 `answer`、`has_evidence`、结构化 `sources`，每项包含 article_id/title/version_no/chunk_index/heading_path。
sources 由程序从验证过的证据构造，不从模型生成文本解析；前端用受控文章 ID 打开 `/article/:id`。
公开链接打开点击时的当前公开版本，source.version_no 表示回答时使用的版本，二者可能随再次发布而变化。
无有效 evidence 直接返回明确的无依据提示，完全不调用 LLM；服务故障返回错误，不伪装成无依据。
有 evidence 时模型仅接收问题和验证后的证据，提示禁止参数知识补全。仍需真实模型 smoke 检查语言事实准确性。

ArticleEdit 默认“版本问答”，只使用应用绑定的已保存 article_id/version_no，忽略浏览器未保存 HTML。
程序先检索 exact version，再生成答案；没有 evidence 不调用模型。LLM 不选择其他文章/版本或调用写工具。
“写作提案”明确选择 `mode=write`，保留全文读取 → 单次提案 → Preview → Human Approval → Apply，
不使用 chunks 替代写作快照；旧 `run()` 不变。低层 `run_with_response()` 的旧写作默认值保留，HTTP 明确传入 mode。
每次知识请求独立，无 conversation_id、checkpoint、Redis 历史或长期记忆。

### 验证与上线门槛

普通 pytest 使用 mock Cloud、内存 Qdrant、FakeEmbedding/FakeModel；不会下载权重或调用 production。
浏览器 E2E 保留真实 Vue/FastAPI/BlogClient/Go/SQLite 链路，RAG 使用内存 Qdrant 与 FakeEmbedding，
写作保留真实 LangGraph/ToolNode；无依据/错误/恶意 source 的部分前端展示场景用路由 mock。
测试不能替代真实 Cloud/Groq/TiDB 的生产语义和质量验证。

Review 通过后统一提交、推送、部署 Go/Python/Frontend，再进行 production smoke：

1. Render 保留 13.5-A 的 Cloud 配置并增加 PUBLISHED_KNOWLEDGE_COLLECTION，运行显式 reconcile。
2. 选取有授权的测试文章，验证首次已发布版本被索引；继续保存工作稿，知识回答仍引用旧公开版本。
3. 发布新版本，先确认 Go Publish 成功，再确认 sync 成功且旧 published points 退出。
4. 用跨两篇文章的问题检查综合回答、sources 及公开链接；用 exact version 检查 ArticleEdit 问答。
5. 验证无 evidence 分支、归档过滤与同步失败恢复；不打印 JWT/密钥。

本次代码/本地测试完成不代表剩余 Task 13.5 已上线；production full E2E 完成前不封板。

### Production E2E 写作回归修复

完整 HTML 提案提交成功后直接返回程序构造的确认说明；不再将快照与提案发给模型生成结束语。
这避免了第三次模型请求因限额、超时或格式错误而使已经生成的提案丢失。没有修改 timeout、token 限额或 SDK 零重试设置。
HTTP 显式 `mode=write` 必须返回结构化 proposal；如果模型只给出普通聊天文本，则返回安全的 `proposal_not_created` 错误，不能冒充编辑成功。
写作仍以完整历史快照为基础，成功仅表示生成提案；Preview、人工确认、Go Apply 均保留。

模型异常使用受控 `detail.code`：`model_rate_limited` / `model_request_rejected` / `model_unavailable` / `model_output_invalid` 为 502，
`model_timeout` 为 504。应用用户额度超限仍是 429 + `Retry-After`，不混淆模型配额与应用限流。
模型请求故障日志只记录固定 stage/code，不输出异常原文、请求正文、JWT、密钥或 traceback。
共享回复策略使用面向用户的产品语言，不主动展示内部工具名；不在前端删除字符串。
