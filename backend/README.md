# Blog System API

一个使用 Go + Gin + GORM 构建的博客系统后端 API。

## 项目特性

- 🔐 JWT 认证与授权
- 📝 文章 CRUD 操作
- 💬 评论系统（支持嵌套回复）
- ❤️ 点赞功能
- ⭐ 收藏功能
- 🔍 文章搜索
- 📄 分页查询
- 🗄️ Redis 缓存（可选，自动降级到内存缓存）
- 👤 用户管理与权限控制

## 技术栈

- **Web框架**: [Gin](https://github.com/gin-gonic/gin)
- **ORM**: [GORM](https://gorm.io)
- **数据库**: SQLite (开发) / MySQL (生产)
- **缓存**: [Redis](https://redis.io) (可选)
- **认证**: [JWT](https://jwt.io)
- **配置**: [Viper](https://github.com/spf13/viper)

## 项目结构

```
blog-system/
├── cmd/server/          # 程序入口
├── internal/
│   ├── config/          # 配置管理
│   ├── database/        # 数据库和缓存初始化
│   ├── handler/         # HTTP 处理器
│   ├── middleware/       # 中间件
│   ├── model/           # 数据模型
│   ├── repository/      # 数据访问层
│   ├── router/          # 路由配置
│   └── service/         # 业务逻辑层
├── pkg/
│   ├── auth/            # JWT 和密码工具
│   └── response/        # 统一响应格式
├── config/              # 配置文件
├── Makefile             # 构建命令
└── README.md            # 项目说明
```

## 快速开始

### 1. 克隆项目

```bash
cd blog-system
```

### 2. 安装依赖

```bash
make deps
# 或者
go mod download
```

### 3. 运行项目

```bash
make run
# 或者
go run ./cmd/server
```

服务器将在 `http://localhost:8080` 启动。

### 4. 健康检查

```bash
curl http://localhost:8080/health
```

## API 文档

### 认证接口

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/auth/register | 用户注册 | 否 |
| POST | /api/v1/auth/login | 用户登录 | 否 |

### 用户接口

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/v1/user/profile | 获取个人信息 | 是 |
| PUT | /api/v1/user/profile | 更新个人信息 | 是 |
| PUT | /api/v1/user/password | 修改密码 | 是 |
| GET | /api/v1/user/favorites | 获取收藏列表 | 是 |

### 文章接口

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/v1/articles | 文章列表 | 否 |
| GET | /api/v1/articles/search | 搜索文章 | 否 |
| GET | /api/v1/articles/:id | 文章详情 | 否 |
| POST | /api/v1/articles | 创建文章 | 是 |
| PUT | /api/v1/articles/:id | 更新文章 | 是 |
| DELETE | /api/v1/articles/:id | 删除文章 | 是 |

### 互动接口

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | /api/v1/articles/:id/like | 点赞 | 是 |
| DELETE | /api/v1/articles/:id/like | 取消点赞 | 是 |
| GET | /api/v1/articles/:id/likes | 点赞数 | 否 |
| POST | /api/v1/articles/:id/comments | 发表评论 | 是 |
| GET | /api/v1/articles/:id/comments | 评论列表 | 否 |
| DELETE | /api/v1/comments/:id | 删除评论 | 是 |
| POST | /api/v1/articles/:id/favorite | 收藏 | 是 |
| DELETE | /api/v1/articles/:id/favorite | 取消收藏 | 是 |
| GET | /api/v1/articles/:id/favorite | 是否收藏 | 是 |

### 管理员接口

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | /api/v1/admin/users | 用户列表 | 管理员 |

## 请求示例

### 用户注册

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "123456"
  }'
```

### 用户登录

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "123456"
  }'
```

### 创建文章

```bash
curl -X POST http://localhost:8080/api/v1/articles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "title": "我的第一篇文章",
    "content": "这是文章内容...",
    "status": "published"
  }'
```

## 文章版本 Diff（Agent API）

`GET /api/v1/agent/articles/:id/diff?from_version=2&to_version=6`

使用既有 Bearer JWT 认证，仅文章本人可读取历史版本，包括其他管理员在内的非所有者返回 403。
参数要求为正整数且 `from_version < to_version`，允许非相邻版本。
文章或任一版本不存在返回 404，参数非法返回 400，内部错误返回不含数据库细节的 500。
先校验文章归属，再以 `article_id + version_no` 查询两个快照；不跨文章查找同号版本。
读取不改变 Article、ArticleVersion、Version、PublishedVersion、标签、计数或缓存。

成功响应仍使用 `{code, message, data}` 信封。`data` 示例：

```json
{
  "article_id": 18,
  "from_version": 2,
  "to_version": 6,
  "field_changes": {
    "title": {"changed": false, "before": "Redis", "after": "Redis"},
    "summary": {"changed": false, "before": "", "after": ""},
    "cover_image": {"changed": false, "before": "", "after": ""}
  },
  "content": {
    "changed": true,
    "changes": [{
      "operation": "modify",
      "before_index": 0,
      "after_index": 0,
      "before": {"type": "paragraph", "text": "Redis很快"},
      "after": {"type": "paragraph", "text": "Redis性能很高"}
    }]
  }
}
```

只比较标题、摘要、封面和正文；status、tags 不在快照 Diff 中。
正文通过已有 `golang.org/x/net/html` 解析后提取有序的 heading、paragraph、list_item、
blockquote 和 code_block，不对原始 HTML 字符串做 Diff，不使用 LLM。
列表项和引用中的段落继承容器类型，嵌套内容按文档顺序展开，不重复复制父节点文本。
普通文本合并空白并解码实体；代码块保留缩进和换行，统一 CRLF/CR 为 LF。
script、style、template 和注释不作为正文文本。

使用 Hirschberg LCS 对齐 `{type, text}` 完全相同的块；并列最优解选择最早的目标分割点。
相邻未匹配区域按位置配对，同类型块返回 modify，不同类型用 delete/insert 表示。
移动块表现为删除和插入，不提供 move 操作或字词级 Diff。
索引是各自完整归一化序列中的 0 起始位置；insert 的 before/before_index 为 null，
delete 的 after/after_index 为 null。不返回未变块，没有内容变化时 changes 为 `[]`。
序列比较最坏时间复杂度为 O(n×m)，采用线性行存储避免完整二维矩阵。

初版限制：忽略粗体等行内格式、HTML 属性、标题级别和列表/引用嵌套深度；
正文图片、嵌入媒体和表格结构尚不支持完整语义比较。
例如 `<p>Redis</p>` 到 `<p><strong>Redis</strong></p>` 可视为不变。
因此 content.changed 表示归一化语义变化，不等同于快照原始 HTML 字节变化。
返回文本应作为纯文本转义展示，不应直接作为 HTML 执行。

## HTML 结构化分块（Task 09）

`internal/chunking` 提供纯函数，只将 HTML 转换成结构化 Chunk，不访问数据库、调用 LLM，
也不提供 HTTP API、Agent Tool、向量字段或索引写入。调用方负责取得已授权的文章内容。

```go
import "blog-system/internal/chunking"

options := chunking.DefaultOptions()
chunks, err := chunking.ChunkHTML(articleHTML, options)
// 处理 err 后，可按需调用 chunking.RenderText(chunks[i]) 获取纯文本。
```

每个 Chunk 只保存 `index`、`heading_path` 和 `blocks`。
Heading 包含 `level`（1～6）和 `text`，正文块包含 `type` 和 `text`；不保存 DOM、属性或重复的 Content 字段。
标题更新采用栈：移除当前路径中级别数值大于或等于新标题的节点，再追加新标题。
没有标题的正文使用空路径；跳级标题不生成虚构的中间层级。每次出现标题都开始新的结构分组，
相同标题文字再次出现也视为新的段落范围；只有标题而没有正文的分组不生成 Chunk。

默认尺寸配置：`TargetSize=800`、`MaxSize=1200`、`MaxOverflow=120`。
配置要求 `0 < TargetSize <= MaxSize`，溢出额度非负且与上限相加不会产生整数溢出。
尺寸通过集中辅助函数按 rune 计算，计入正文块间的两个换行符，不计入 HeadingPath。
因此渲染后的完整文本可能比正文尺寸更长；未来采用 token 预算时需另行评估标题上下文开销。

- 同一标题路径内优先保留、合并完整块。达到 TargetSize 后通常沿块边界结束；
  若剩余尾部可以在 MaxSize 内全部合并，则继续合并，避免不必要的小尾块。
- 多个块的合并不超过 MaxSize；单个块在 MaxSize + MaxOverflow 内可保持完整，并独占 Chunk。
- 超出该阈值的普通块优先按中英文句末标点切分，最后按 rune 兜底。
  代码块改用行边界，只有过长的单行才按 rune 兜底，不分析编程语言。
- 优先使用 MaxSize 内最后一个句/行边界；若没有，则可使用溢出额度内的首个边界；
  仍没有边界才按 MaxSize 个 rune 切分。最后一片可在溢出额度内保留。
- 所有片段保持顺序，标点、空白及代码换行归属原片段；片段直接拼接等于归一化原块。
  文本 overlap 为 0，仅通过 HeadingPath 提供结构上下文。

共享的 `internal/htmlcontent` 负责解析与遍历，保留标题级别，但不依赖 Diff 或 Chunker DTO。
Chunker 使用文档解析，忽略 head、script、style、template 和注释；
Diff 保留原有片段解析入口和模型，继续忽略标题级别，Task 08 的比较语义不变。

v1 限制：普通文本空白归一化，代码保留缩进和换行（CRLF/CR 统一为 LF）；
不保留行内格式、HTML 属性、列表编号或列表/引用的嵌套深度。
图片、嵌入媒体和表格结构没有专用模型，不保证完整表达这些元素。
句末判断是确定性标点规则，可能把缩写或小数中的句点当作边界；rune 兜底不保证 Unicode 字素簇完整。
空正文块被忽略，标题本身只存入路径。`RenderText` 按需连接路径和正文，不是原 HTML 的无损还原。
渲染结果仍应作为纯文本处理，不应直接当作 HTML 执行。

## 默认账号

系统首次启动时会自动创建默认管理员账号：

- **用户名**: admin
- **密码**: admin123456

## 配置说明

配置文件位于 `internal/config/config.yml`：

```yaml
app:
  name: blog-system
  port: ":8080"
  mode: debug

database:
  driver: sqlite
  dsn: blog.db

redis:
  host: localhost
  port: "6379"
  password: ""
  db: 0

jwt:
  secret: your-secret-key
  expiration: 24
```

## 开发命令

```bash
# 编译项目
make build

# 运行项目
make run

# 运行测试
make test

# 格式化代码
make fmt

# 代码检查
make vet

# 清理构建文件
make clean
```

## 响应格式

### 成功响应

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

### 分页响应

```json
{
  "code": 200,
  "message": "success",
  "data": [ ... ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 100,
    "pages": 10
  }
}
```

### 错误响应

```json
{
  "code": 400,
  "message": "错误信息"
}
```

## License

MIT License
## Task 10：Agent 历史版本分块 API

`GET /api/v1/agent/articles/:id/versions/:version/chunks` 复用 JWT 中间件及文章所有权规则。
仅所有者可读取，其他用户（包括其他管理员）返回 403；文章、历史版本缺失或文章已删除返回 404；
非正整数 ID/版本返回 400。读取不增加浏览量，不更新 Article / ArticleVersion / PublishedVersion。

Service 通过 Repository 获取指定不可变快照，调用现有 Task 09 `ChunkHTML` 默认选项和 `RenderText`。
成功响应沿用 `{code, message, data}`，`data` 为：

```json
{
  "user_id": 7,
  "article_id": 18,
  "version_no": 1,
  "chunks": [{
    "index": 0,
    "heading_path": [{"level": 1, "text": "Redis"}],
    "blocks": [{"type": "paragraph", "text": "正文"}],
    "text": "Redis\n\n正文"
  }]
}
```

`user_id` 来自服务端验证后的 JWT，用于 Python 的 Qdrant 检索隔离，不能由请求覆盖。
`text` 仅是 API 派生表示，未添加到 Task 09 内部 Chunk 模型或数据库；空分块返回 `[]`。
Python 只能通过此 API 获得分块；本接口不自动建立或更新任何向量索引。
本地 Qdrant、embedding 和显式索引操作见 [Agent Service README](../agent-service/README.md#task-10指定文章版本的-dense-rag)。

## Task 11：读取编辑基准历史版本

`GET /api/v1/agent/articles/:id/versions/:version` 为 Agent 编辑提案提供完整的历史内容。
继承 Agent JWT 中间件，Service 复用 Repository 的 `FindOwnedVersion`，先校验文章归属，再读取该文章的指定快照。
即使当前 Article 已更新或存在其他 PublishedVersion，也只返回请求的历史版本，不回退到工作内容或其他文章。

响应沿用 `{code, message, data}`，`data` 包含：

```json
{
  "article_id": 23,
  "version_no": 7,
  "title": "Redis",
  "content": "<h1>Redis</h1><p>该历史版本的完整 HTML。</p>",
  "summary": "摘要",
  "cover_image": "/cover.png"
}
```

401 表示身份无效；非所有者（包括其他管理员）返回 403；文章/版本不存在或文章已删除返回 404；
非正整数或超出范围的 ID/版本返回 400。未增加提案写接口或数据表。
此读取不修改正文、Article.Version、PublishedVersion、标签、计数或版本快照，也不增加浏览量或失效缓存。
Python 在读取原文和提交运行内提案时均调用此 GET；提案只存在于 Python 的本次运行结果中。
Task 12 的应用操作见下文，Agent 工具本身仍不能应用或发布提案。

## Task 12：人工批准与安全应用

应用直接调用 Go 的认证接口，无需经由 LangGraph 或启动 Python/RAG 服务：

| 操作 | 路由 |
| --- | --- |
| 预览 | `POST /api/v1/articles/:id/edit-proposal/preview` |
| 显式批准并应用 | `POST /api/v1/articles/:id/edit-proposal/apply` |

两者均要求 `Authorization: Bearer <用户 JWT>`，请求体仅允许：

```json
{
  "base_version_no": 7,
  "proposed_content": "<h1>Redis</h1><p>经人工审核的完整正文。</p>"
}
```

文章 ID 来自路由，身份来自 JWT。拒绝未知字段，包括 `user_id`、`owner_id`、`article_id`、
`new_version_no`、`published_version`、`status`、`tag_ids`、`approved`、`change_summary`。
从 Task 11 提案构建请求时只选取上述两个字段；提案摘要用于展示，不是业务授权依据。
明确调用 Apply 即表示批准这份请求中的正文；预览本身不批准、不签发凭据，也不是 Apply 的服务端前置状态。
拒绝提案只需丢弃客户端数据。没有 proposal 表、ID、审批历史或审批状态。

预览先通过 Go 校验文章所有权，再加载指定历史快照，复用 Task 08 的
`compareArticleVersions → normalizeArticleHTML → compareContentBlocks`。
成功响应沿用 `{code, message, data}`，其中预览的 `data` 为：

```json
{
  "article_id": 23,
  "base_version_no": 7,
  "field_changes": {
    "title": {"changed": false, "before": "Redis", "after": "Redis"},
    "summary": {"changed": false, "before": "", "after": ""},
    "cover_image": {"changed": false, "before": "", "after": ""}
  },
  "content": {
    "changed": true,
    "changes": [{
      "operation": "modify", "before_index": 1, "after_index": 1,
      "before": {"type": "paragraph", "text": "原正文。"},
      "after": {"type": "paragraph", "text": "经人工审核的完整正文。"}
    }]
  }
}
```

`field_changes`、`content` 与 Task 08 的 DTO 完全相同。提案不是已存在的版本，因此元数据使用
`base_version_no`，不虚构 `to_version`；原有历史版本 diff API 的响应不变。
预览只比较正文，标题/摘要/封面保持基础快照值，状态和标签不参加 diff。
允许预览旧版本，但应用时旧版本必须返回冲突，不能自动合并或变基。

Apply 的 Repository 事务先重新锁定当前文章、校验所有者，然后调用 Task 03 的
`checkExpectedVersion`，要求 `Article.Version == base_version_no`，检查归档状态及基础快照存在，
执行共同正文校验和无变化检查，再仅替换事务中读取的当前正文。
复用人工编辑的 `saveArticleContent`：保存 Article、递增 Version、写入完整 ArticleVersion，
同一个 `tx` 提交或回滚。不更新标签；保留 Title/Summary/CoverImage、Article.Status 和 PublishedVersion。
快照 `CreatedBy` 为批准用户，`Source=user`。缓存只在成功提交后失效。

Apply 成功的 `data` 示例：

```json
{
  "article_id": 23,
  "previous_version_no": 7,
  "new_version_no": 8,
  "status": "draft",
  "published_version": 6
}
```

这里 `status=draft` 表示此次操作保存了未发布的工作版本，不是覆盖文章生命周期。
原本已发布的文章仍为 `Article.Status=published`，公开读取继续使用 V6；新 V8 不会自动公开。
原本草稿仍为草稿；已归档文章拒绝编辑。响应版本来自此次事务结果，不在提交后重新查询。

| 情况 | HTTP / message |
| --- | --- |
| 缺少或无效 JWT | 401，沿用认证中间件 |
| 非所有者（包括其他管理员） | 403 |
| 文章不存在、已删除、基础快照缺失 | 404 |
| 基础版本与当前版本不同 | 409，沿用 Task 03 版本冲突信息；优先于无变化检查 |
| 文章已归档 | 409 |
| 非法请求 / 空正文 | 400 |
| 正文完全相同 | 400，`文章正文没有实际变化` |
| 数据库等内部失败 | 500，固定错误信息，不返回数据库细节 |

无变化检查按原有写入规则比较原始正文字节，不改变 HTML。
Task 08 的结构化预览忽略部分格式差异，因此格式修改可能预览无块差异但 Apply 仍生成内容版本。
人工写入与 Apply 共用 `ValidateArticleContent` 的非空规则；这不是 HTML sanitizer。
现有 Go 写入没有完整 XSS/URL/CSS 安全清洗，Task 11 的基础校验也不能作为可信安全保证。
生成及预览 HTML 仍是不可信输入；Task 13 展示时需要安全渲染与明确批准交互，生产使用前应单独完善统一 HTML 安全策略。
本任务不新增独立 sanitizer，也不声称任意 HTML 已安全。

Apply 不调用 Python、LLM、embedding、Qdrant 或 IndexArticleVersion。
Task 10 的索引保持独立显式操作。LangGraph 的模型绑定和 ToolNode 均未增加 Apply 能力；
未来编辑 UI 必须使用 Task 11 的 `run_with_response()`，不要用带旧草稿写工具的兼容 `run()` 替代。

测试覆盖历史基准预览、与 Task 08 diff 对照、所有权及 JWT、严格请求字段、重放冲突、
草稿/已发布/归档行为、无变化及空正文、缓存和历史快照不变，以及真实 SQLite 触发器回滚。
回滚分别在 `BEFORE UPDATE articles` 和 `AFTER INSERT article_versions` 注入失败，检查文章、
版本、公开指针、时间戳和标签全部恢复，移除触发器后重试连续生成下一版。
SQLite 测试不等同于 MySQL 并发集成测试；MySQL 行锁继续沿用既有实现。
# Task 13：前端版本列表接线

新增 `GET /api/v1/user/articles/:id/versions?page=1&limit=20`，继承 JWT/当前所有者校验，
返回分页元数据及 `{article_id, version_no, title, created_at}` 列表，按版本号降序。
不返回全文，不增加浏览量或写入业务状态。前端选择历史快照仍复用 Task 11 单版本 GET，
历史比较复用 Task 08 Diff，批准仍复用 Task 12 Preview/Apply；版本和生命周期写逻辑不变。

浏览器 E2E fixture 为 `TestTask13E2EServer`，仅在 `TASK13_E2E=1` 时启动，
使用测试内存 SQLite/测试身份并绑定 127.0.0.1:18080。普通 `go test ./...` 跳过服务器 fixture，
正常运行新增的版本列表权限、分页及只读测试。此 fixture 不读取应用数据库配置，也不用于部署。
