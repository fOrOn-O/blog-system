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
