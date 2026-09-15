# Blog Agent Service

Task 05 的 FastAPI 服务骨架，当前仅提供进程健康检查与集中配置。
Python 后续通过 HTTP 调用 Go Backend，由 Go 执行认证授权、业务规则和数据库事务。
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
FastAPI、Uvicorn 和 pydantic-settings 是运行依赖；pytest、httpx 仅用于测试。
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

## 测试

```bash
uv run --locked pytest
```

测试使用 FastAPI TestClient，覆盖健康响应、应用配置、默认配置、`.env` 加载、
环境变量优先级和日志级别校验，不需要启动外部服务。
