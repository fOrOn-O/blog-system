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
