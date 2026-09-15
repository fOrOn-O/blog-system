# Blog Agent Service

FastAPI 服务骨架，提供进程健康检查、集中配置和异步 BlogClient。
BlogClient 通过 HTTP 调用 Go Agent API，由 Go 执行认证授权、业务规则和数据库事务。
本服务不连接博客数据库，也不依赖 Go Backend 在线才能启动。

## 安装

要求 Python 3.12 或更高稳定版本，并已安装 [uv](https://docs.astral.sh/uv/getting-started/installation/)。
选择 3.12 作为下限，以使用成熟的 Python 版本并保留后续升级空间。

在仓库根目录执行：

```bash
cd agent-service
uv sync --locked
```

该命令创建本地 `.venv`，安装服务以及默认的 `dev` 依赖组。
FastAPI、Uvicorn、pydantic-settings、Pydantic 和 httpx 是运行依赖；pytest 用于测试。
Hatchling 仅用于构建 Python 包。依赖版本记录在 `uv.lock` 中。

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

环境变量优先于当前目录的 `.env`，未设置时使用默认值。
配置加载后会缓存，修改后需要重启进程；不输出完整配置或环境变量。
`.env` 已被 Git 忽略，示例配置不包含密钥。

```bash
uv run --locked uvicorn app.main:app --host 127.0.0.1 --port 8000
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

```bash
uv run --locked pytest
```

测试使用 FastAPI TestClient，覆盖健康响应、应用配置、默认配置、`.env` 加载、
环境变量优先级和日志级别校验。BlogClient 测试使用 httpx.MockTransport，覆盖六个业务接口、
身份隔离、请求与响应契约、错误映射、无重试、连接关闭和令牌脱敏，不需要启动 Go 或数据库。
