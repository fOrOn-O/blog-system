# Blog Agent Service

FastAPI 服务提供进程健康检查、集中配置、异步 BlogClient、LangGraph Single Agent 和版本内 Dense RAG。
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
只在本地 E5 环境安装。远程 embedding 环境可省略 `--extra local`，不因此安装 PyTorch 和模型运行库。
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
`AgentContext` 携带当前 run 的 JWT、BlogClient 和可选 RAG 服务；ToolRuntime 在执行工具时注入它，
JWT 不写入消息、普通 State、Prompt 或模型工具参数，也不出现在 Context 的 repr 中。

模型绑定与 ToolNode 执行使用同一份六工具白名单：

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

## Task 10：指定文章版本的 Dense RAG

```text
显式 index → BlogClient → Go JWT / 所有权校验 → ArticleVersion → Task 09 ChunkHTML / RenderText
          → EmbeddingProvider.embed_documents → Qdrant 按版本替换索引

用户问题 → 现有 LangGraph → search_article_version → Go 再次校验所有权及版本
         → EmbeddingProvider.embed_query → Qdrant Top-K → 校对 Go 原文
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

`EmbeddingProvider` 协议位于 `app/rag/embedding.py`，提供 `dimension`、`profile`、
`embed_documents` 和 `embed_query`。索引和检索只依赖该协议，不依赖 SentenceTransformer。
本地实现使用 `passage: <chunk text>` 和 `query: <question>`，并归一化向量；
前缀只存在于推理输入，原文、Go Chunk 和 Qdrant payload 都不会被添加前缀。

集合创建时写入 metadata：

```json
{"embedding_profile":{"provider":"local_e5","model":"intfloat/multilingual-e5-small","dimension":384,"distance":"Cosine","collection":"article_chunks_e5_v1"}}
```

每次读写都核对实际 size、distance 和完整 profile。即使维度相同，只要模型或 provider 不同就拒绝使用；
现有无 profile 的集合也不会被自动认领或删除。更换模型时必须使用新集合、重新显式索引。
Provider 和 Store 在组装服务时也必须具有完全一致的 profile；未知 provider 明确报错，不回退到本地 E5。

一个 Go Chunk 对应一个 Point。ID 为固定 UUID namespace 下
`blog-system/article/{article_id}/version/{version_no}/chunk/{chunk_index}` 的 UUIDv5。
payload 仅包含：`user_id`、`article_id`、`version_no`、`chunk_index`、`heading_path`、`text`；
前三个字段建立整数 payload index。没有 HTML DOM，也不保存完整 Blocks。
检索与删除均使用 `user_id AND article_id AND version_no`，不进行跨文章检索。

重建顺序：获取所有 Go chunks → 生成所有向量 → 校验数量、维度、有限非零值 →
校验集合 profile → 删除该用户/文章/版本旧点 → 批量 upsert 新点。
因此旧 chunk 3 在新结果仅有 0～2 时会被清除，空结果会清空该版本索引，重复索引不会增加逻辑点数。
embedding 失败发生在删除前；删除成功而 upsert 失败时可能暂时缺少索引，显式重试可修复。
该过程不是数据库事务，不自动重试写操作；当前仅在单进程内串行化替换操作，未提供跨进程写入锁。

### Render / 生产配置边界

Render 应通过环境变量配置 `APP_ENV=production`、外部 `QDRANT_URL` / `QDRANT_API_KEY`，
以及一整组远程 provider / model / dimension / distance / collection 配置。
不能只改模型名称却继续使用旧集合，也不应将 Qdrant 数据保存到 Render 临时磁盘。

**本任务只实现本地 `LocalE5EmbeddingProvider`，没有实现任何真实远程 embedding API 适配器。**
生产环境不会自动启动本地模型：`APP_ENV=production` 下的 `local_e5` 会明确报错；
未实现的远程 provider 同样明确报错。`/health` 仍只是进程存活检查，不代表 RAG readiness。

真正部署前仍需选择远程服务，编写一个实现相同 `EmbeddingProvider` 协议的 HTTP adapter，
在 `create_embedding_provider` 注册并加入该服务所需的集中配置、凭据和契约测试。
索引、检索和 LangGraph 逻辑无需重写。随后配置外部持久化 Qdrant、Go 服务地址及密钥，
验证实际 embedding profile、网络连通性、权限和生产容量，并显式重建新集合。
如远程服务运行同一 E5 模型，前缀规则也应由该 adapter 负责，不放到通用索引逻辑中。

### 测试与当前限制

```powershell
& 'D:\Anaconda\python.exe' -m pytest
```

新增测试使用 FakeEmbedding、FakeModel、httpx.MockTransport 和 Qdrant SDK 内存模式；
覆盖前缀、单次加载、profile/实际集合规格校验、点身份、重建、失败保护、三重过滤、Top-K、
Go 原文校对及真实 LangGraph/ToolNode/JWT Context 回路。内存模式不实际建立 payload 索引，
测试另外校验建索引请求；不能据此宣称已验证 Docker 网络或真实模型检索质量。

当前只支持单个指定文章版本的 Dense Top-K。未索引时返回空结果，不自动建索引；
空结果不证明该版本没有相关内容，Agent 应说明没有可用检索依据。
无相似度阈值或 reranker；上下文保持 Qdrant 返回顺序，最终自然语言生成仍依赖模型。
Go Task 09 按字符/语义块分块，而 E5 超过模型的 512 token 上限时会截断 embedding 输入，
详见 [E5 模型说明](https://huggingface.co/intfloat/multilingual-e5-small)。原文仍完整保存；本任务不增加 token 重分块。
每次检索为保证 Go 是权限与内容真源会重新获取整份版本分块，当前以正确性优先，尚未优化该开销。
