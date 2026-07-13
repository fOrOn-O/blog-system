# Blog System 开发文档

> 版本：v1.0.0 | 最后更新：2025-07-12

---

## 目录

- [1. 项目概述](#1-项目概述)
- [2. 快速开始](#2-快速开始)
- [3. 系统架构](#3-系统架构)
- [4. 数据库设计](#4-数据库设计)
- [5. API 接口文档](#5-api-接口文档)
- [6. 配置说明](#6-配置说明)
- [7. 开发指南](#7-开发指南)
- [8. 部署指南](#8-部署指南)
- [9. 常见问题](#9-常见问题)

---

## 1. 项目概述

### 1.1 简介

Blog System 是一个基于 Go 语言构建的博客系统后端 API，提供用户认证、文章管理、评论互动、点赞收藏等完整功能。

### 1.2 技术栈

| 组件 | 技术 | 版本 | 说明 |
|------|------|------|------|
| Web 框架 | Gin | v1.9.1 | HTTP 路由与中间件 |
| ORM | GORM | v1.25.5 | 数据库操作 |
| 数据库 | SQLite / MySQL | - | 开发用 SQLite，生产用 MySQL |
| 缓存 | Redis | v8.11.5 | 可选，支持内存降级 |
| 认证 | JWT | v5.2.0 | 基于 HMAC-SHA256 |
| 配置 | Viper | v1.18.2 | YAML 配置文件 |

### 1.3 功能清单

- ✅ 用户注册 / 登录 / 个人信息管理
- ✅ JWT 认证与授权（管理员 / 普通用户）
- ✅ 文章 CRUD（创建、查询、更新、删除）
- ✅ 文章分页列表与关键词搜索
- ✅ 评论系统（支持嵌套回复，最多2层）
- ✅ 点赞 / 取消点赞
- ✅ 收藏 / 取消收藏
- ✅ Redis 缓存 + 内存缓存自动降级
- ✅ 优雅关闭

---

## 2. 快速开始

### 2.1 环境要求

| 工具 | 最低版本 | 说明 |
|------|----------|------|
| Go | 1.21+ | [下载](https://go.dev/dl/) |
| Git | 任意 | [下载](https://git-scm.com/) |
| Redis（可选） | 6.0+ | 缓存服务，不安装则使用内存缓存 |

### 2.2 安装与运行

```bash
# 1. 进入项目目录
cd blog-system

# 2. 下载依赖
go mod download

# 3. 运行项目
go run ./cmd/server
```

服务器启动后输出：

```
===================================
  Blog System API Server
  Port: :8080
  Mode: debug
===================================
  Health: http://localhost:8080/health
  API:    http://localhost:8080/api/v1
===================================
默认管理员账号已创建: admin / admin123456
```

### 2.3 验证服务

```bash
# 健康检查
curl http://localhost:8080/health

# 响应示例
{
  "code": 200,
  "message": "success",
  "data": {
    "status": "ok",
    "message": "Blog System API is running"
  }
}
```

---

## 3. 系统架构

### 3.1 分层架构

```
┌─────────────────────────────────────────────────────────┐
│                     Client (前端)                        │
└─────────────────────────┬───────────────────────────────┘
                          │ HTTP
┌─────────────────────────▼───────────────────────────────┐
│                   Router (路由层)                         │
│              internal/router/router.go                   │
├─────────────────────────────────────────────────────────┤
│                 Middleware (中间件)                       │
│         auth.go │ cors.go │ logger.go                   │
├─────────────────────────────────────────────────────────┤
│                 Handler (处理器层)                        │
│    解析请求参数 → 调用 Service → 返回响应                 │
├─────────────────────────────────────────────────────────┤
│                 Service (业务逻辑层)                      │
│    参数校验 → 业务处理 → 缓存策略 → 数据转换              │
├─────────────────────────────────────────────────────────┤
│                Repository (数据访问层)                    │
│    封装数据库 CRUD 操作                                   │
├─────────────────────────────────────────────────────────┤
│                   Model (模型层)                          │
│    定义数据结构与表映射                                    │
├─────────────────────────┬───────────────────────────────┤
│        Database         │           Redis               │
│       (SQLite/MySQL)    │      (可选，内存降级)          │
└─────────────────────────┴───────────────────────────────┘
```

### 3.2 目录结构

```
blog-system/
├── cmd/
│   └── server/
│       └── main.go                  # 程序入口
├── internal/                        # 私有代码（不可被外部导入）
│   ├── config/
│   │   ├── config.go                # 配置结构体与加载
│   │   └── config.yml               # 配置文件
│   ├── database/
│   │   ├── database.go              # 数据库初始化
│   │   └── redis.go                 # Redis + 内存缓存
│   ├── handler/                     # HTTP 处理器
│   │   ├── auth_handler.go          # 认证接口
│   │   ├── user_handler.go          # 用户接口
│   │   ├── article_handler.go       # 文章接口
│   │   ├── comment_handler.go       # 评论接口
│   │   ├── like_handler.go          # 点赞接口
│   │   ├── favorite_handler.go      # 收藏接口
│   │   └── utils.go                 # 工具函数
│   ├── middleware/                   # 中间件
│   │   ├── auth.go                  # JWT 认证
│   │   ├── cors.go                  # 跨域
│   │   └── logger.go                # 日志
│   ├── model/                       # 数据模型
│   │   ├── user.go
│   │   ├── article.go
│   │   ├── comment.go
│   │   ├── like.go
│   │   └── favorite.go
│   ├── repository/                  # 数据访问层
│   │   ├── user_repo.go
│   │   ├── article_repo.go
│   │   ├── comment_repo.go
│   │   ├── like_repo.go
│   │   └── favorite_repo.go
│   ├── router/
│   │   └── router.go               # 路由注册
│   └── service/                     # 业务逻辑层
│       ├── auth_service.go
│       ├── user_service.go
│       ├── article_service.go
│       ├── comment_service.go
│       ├── like_service.go
│       └── favorite_service.go
├── pkg/                             # 公共工具包（可被外部导入）
│   ├── auth/
│   │   ├── jwt.go                   # JWT 工具
│   │   └── password.go              # 密码加密
│   └── response/
│       └── response.go              # 统一响应
├── docs/
│   └── DEVELOPMENT.md               # 本文档
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 3.3 请求处理流程

```
Client Request
     │
     ▼
  Router ──────────────► 匹配路由
     │
     ▼
  Middleware ───────────► CORS → Logger → Auth
     │
     ▼
  Handler ─────────────► 解析参数 (ShouldBindJSON)
     │
     ▼
  Service ─────────────► 业务逻辑 (校验、缓存、转换)
     │
     ▼
  Repository ──────────► 数据库操作 (GORM)
     │
     ▼
  Response ◄──────────── 统一 JSON 格式返回
```

---

## 4. 数据库设计

### 4.1 ER 图

```
┌───────────┐       ┌───────────┐       ┌───────────┐
│   users   │       │  articles │       │   tags    │
├───────────┤       ├───────────┤       ├───────────┤
│ id (PK)   │◄──┐   │ id (PK)   │◄──┐   │ id (PK)   │
│ username  │   │   │ title     │   │   │ name      │
│ email     │   │   │ content   │   │   └─────┬─────┘
│ password  │   │   │ summary   │   │         │
│ avatar    │   │   │ user_id(FK)──┘   │  article_tags
│ bio       │   │   │ view_count │      │  (多对多)
│ role      │   │   │ like_count │      │
│ is_active │   │   │ comment_count   │
│ created_at│   │   │ status    │
│ updated_at│   │   │ created_at│
│ deleted_at│   │   │ updated_at│
└─────┬─────┘   │   │ deleted_at│
      │         │   └─────┬─────┘
      │         │         │
      │         │   ┌─────▼─────┐   ┌───────────┐
      │         │   │ comments  │   │   likes   │
      │         │   ├───────────┤   ├───────────┤
      │         ├───│ user_id   │   │ id (PK)   │
      │         │   │ article_id├───│ user_id   │
      │         │   │ parent_id │   │ article_id│
      │         │   │ content   │   │ created_at│
      │         │   │ created_at│   └───────────┘
      │         │   │ updated_at│
      │         │   │ deleted_at│   ┌───────────┐
      │         │   └───────────┘   │ favorites │
      │         │                   ├───────────┤
      └─────────────────────────────│ user_id   │
                                    │ article_id│
                                    │ created_at│
                                    └───────────┘
```

### 4.2 表结构

#### users 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY, AUTO INCREMENT | 用户ID |
| username | VARCHAR(50) | UNIQUE, NOT NULL | 用户名 |
| email | VARCHAR(100) | UNIQUE, NOT NULL | 邮箱 |
| password | VARCHAR(100) | NOT NULL | 密码（bcrypt 加密） |
| avatar | VARCHAR(255) | | 头像URL |
| bio | TEXT | | 个人简介 |
| role | VARCHAR(10) | DEFAULT 'user' | 角色：user / admin |
| is_active | BOOLEAN | DEFAULT true | 是否激活 |
| created_at | DATETIME | | 创建时间 |
| updated_at | DATETIME | | 更新时间 |
| deleted_at | DATETIME | INDEX | 软删除时间 |

#### articles 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY | 文章ID |
| title | VARCHAR(200) | NOT NULL | 标题 |
| content | TEXT | NOT NULL | 内容 |
| summary | VARCHAR(500) | | 摘要 |
| cover_image | VARCHAR(255) | | 封面图URL |
| user_id | INTEGER | INDEX, FK → users.id | 作者ID |
| view_count | INTEGER | DEFAULT 0 | 浏览量 |
| like_count | INTEGER | DEFAULT 0 | 点赞数 |
| comment_count | INTEGER | DEFAULT 0 | 评论数 |
| status | VARCHAR(20) | DEFAULT 'published' | 状态：draft / published / archived |
| created_at | DATETIME | | 创建时间 |
| updated_at | DATETIME | | 更新时间 |
| deleted_at | DATETIME | INDEX | 软删除时间 |

#### comments 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY | 评论ID |
| content | TEXT | NOT NULL | 内容 |
| user_id | INTEGER | INDEX, FK → users.id | 评论者ID |
| article_id | INTEGER | INDEX, FK → articles.id | 文章ID |
| parent_id | INTEGER | INDEX, FK → comments.id | 父评论ID（NULL为顶级评论） |
| created_at | DATETIME | | 创建时间 |
| updated_at | DATETIME | | 更新时间 |
| deleted_at | DATETIME | INDEX | 软删除时间 |

#### likes 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY | 点赞ID |
| user_id | INTEGER | INDEX, FK → users.id | 用户ID |
| article_id | INTEGER | INDEX, FK → articles.id | 文章ID |
| created_at | DATETIME | | 创建时间 |

#### favorites 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY | 收藏ID |
| user_id | INTEGER | INDEX, FK → users.id | 用户ID |
| article_id | INTEGER | INDEX, FK → articles.id | 文章ID |
| created_at | DATETIME | | 创建时间 |

#### tags 表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | INTEGER | PRIMARY KEY | 标签ID |
| name | VARCHAR(50) | UNIQUE, NOT NULL | 标签名 |

#### article_tags 表（多对多关联）

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| article_id | INTEGER | FK → articles.id | 文章ID |
| tag_id | INTEGER | FK → tags.id | 标签ID |

---

## 5. API 接口文档

### 5.1 通用说明

**Base URL**: `http://localhost:8080/api/v1`

**请求头**:

| Header | 值 | 说明 |
|--------|-----|------|
| Content-Type | application/json | 请求体格式 |
| Authorization | Bearer {token} | JWT 认证令牌（需认证接口） |

**响应格式**:

```json
// 成功
{
  "code": 200,
  "message": "success",
  "data": { ... }
}

// 分页
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

// 错误
{
  "code": 400,
  "message": "错误信息"
}
```

**HTTP 状态码**:

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 500 | 服务器内部错误 |

---

### 5.2 认证接口

#### 5.2.1 用户注册

```
POST /api/v1/auth/register
```

**请求体**:

```json
{
  "username": "testuser",
  "email": "test@example.com",
  "password": "123456"
}
```

| 字段 | 类型 | 必填 | 校验规则 | 说明 |
|------|------|------|----------|------|
| username | string | ✅ | 3-50字符，唯一 | 用户名 |
| email | string | ✅ | 合法邮箱格式，唯一 | 邮箱 |
| password | string | ✅ | 6-72字符 | 密码 |

**成功响应** `201 Created`:

```json
{
  "code": 201,
  "message": "created",
  "data": {
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "avatar": "",
      "bio": "",
      "role": "user"
    },
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**错误响应**:

| 状态码 | 场景 |
|--------|------|
| 400 | 请求参数不合法 |
| 409 | 用户名或邮箱已存在 |

---

#### 5.2.2 用户登录

```
POST /api/v1/auth/login
```

**请求体**:

```json
{
  "username": "testuser",
  "password": "123456"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | ✅ | 用户名 |
| password | string | ✅ | 密码 |

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "avatar": "",
      "bio": "",
      "role": "user"
    },
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**错误响应**:

| 状态码 | 场景 |
|--------|------|
| 400 | 请求参数不合法 |
| 401 | 用户名或密码错误 |
| 401 | 账号已被禁用 |

---

### 5.3 用户接口

> 以下接口均需认证（Authorization: Bearer {token}）

#### 5.3.1 获取个人信息

```
GET /api/v1/user/profile
```

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 1,
    "username": "testuser",
    "email": "test@example.com",
    "avatar": "",
    "bio": "",
    "role": "user"
  }
}
```

---

#### 5.3.2 更新个人信息

```
PUT /api/v1/user/profile
```

**请求体**:

```json
{
  "email": "new@example.com",
  "avatar": "https://example.com/avatar.jpg",
  "bio": "这是我的个人简介"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| email | string | 否 | 新邮箱（需合法格式） |
| avatar | string | 否 | 头像URL |
| bio | string | 否 | 个人简介 |

**成功响应** `200 OK`: 返回更新后的用户信息。

---

#### 5.3.3 修改密码

```
PUT /api/v1/user/password
```

**请求体**:

```json
{
  "old_password": "123456",
  "new_password": "654321"
}
```

| 字段 | 类型 | 必填 | 校验规则 | 说明 |
|------|------|------|----------|------|
| old_password | string | ✅ | | 旧密码 |
| new_password | string | ✅ | 6-72字符 | 新密码 |

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "密码修改成功"
  }
}
```

---

#### 5.3.4 获取收藏列表

```
GET /api/v1/user/favorites?page=1&limit=10
```

**查询参数**:

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| limit | int | 10 | 每页数量（1-100） |

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "article": {
        "id": 5,
        "title": "文章标题",
        "content": "...",
        "summary": "...",
        "user": { ... },
        "view_count": 100,
        "like_count": 10,
        "comment_count": 5,
        "status": "published",
        "created_at": "2025-07-12T10:00:00Z",
        "updated_at": "2025-07-12T10:00:00Z"
      },
      "created_at": "2025-07-12T12:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 1,
    "pages": 1
  }
}
```

---

### 5.4 文章接口

#### 5.4.1 文章列表（公开）

```
GET /api/v1/articles?page=1&limit=10&status=published
```

**查询参数**:

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| limit | int | 10 | 每页数量 |
| status | string | published | 筛选状态：draft / published / archived |

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "title": "第一篇文章",
      "content": "文章正文...",
      "summary": "文章摘要...",
      "cover_image": "",
      "user": {
        "id": 1,
        "username": "admin",
        "email": "admin@blog.com",
        "avatar": "",
        "bio": "",
        "role": "admin"
      },
      "view_count": 50,
      "like_count": 5,
      "comment_count": 3,
      "status": "published",
      "tags": ["Go", "Gin"],
      "created_at": "2025-07-12T10:00:00Z",
      "updated_at": "2025-07-12T10:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 50,
    "pages": 5
  }
}
```

---

#### 5.4.2 搜索文章（公开）

```
GET /api/v1/articles/search?keyword=Go&page=1&limit=10
```

**查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| keyword | string | ✅ | 搜索关键词（匹配标题和内容） |
| page | int | 否 | 页码，默认 1 |
| limit | int | 否 | 每页数量，默认 10 |

---

#### 5.4.3 文章详情（公开）

```
GET /api/v1/articles/:id
```

**路径参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| id | int | 文章ID |

**成功响应** `200 OK`: 返回完整文章信息（含作者、标签）。

**错误响应**:

| 状态码 | 场景 |
|--------|------|
| 400 | 无效的文章ID |
| 404 | 文章不存在 |

---

#### 5.4.4 创建文章（需认证）

```
POST /api/v1/articles
```

**请求体**:

```json
{
  "title": "文章标题",
  "content": "文章正文内容...",
  "summary": "可选摘要",
  "status": "published",
  "tags": ["Go", "Gin"]
}
```

| 字段 | 类型 | 必填 | 校验规则 | 说明 |
|------|------|------|----------|------|
| title | string | ✅ | 最长200字符 | 标题 |
| content | string | ✅ | | 正文 |
| summary | string | 否 | | 摘要（不填自动生成前200字） |
| status | string | 否 | draft / published / archived | 状态，默认 published |
| tags | []string | 否 | | 标签列表 |

**成功响应** `201 Created`: 返回创建的文章。

---

#### 5.4.5 更新文章（需认证，仅作者）

```
PUT /api/v1/articles/:id
```

**请求体**（所有字段可选）:

```json
{
  "title": "新标题",
  "content": "新内容",
  "summary": "新摘要",
  "status": "archived",
  "tags": ["Go", "GORM"]
}
```

**错误响应**:

| 状态码 | 场景 |
|--------|------|
| 400 | 参数错误或无权修改 |
| 404 | 文章不存在 |

---

#### 5.4.6 删除文章（需认证，仅作者）

```
DELETE /api/v1/articles/:id
```

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "文章删除成功"
  }
}
```

---

### 5.5 互动接口

#### 5.5.1 点赞文章（需认证）

```
POST /api/v1/articles/:id/like
```

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "点赞成功"
  }
}
```

**错误响应**:

| 状态码 | 场景 |
|--------|------|
| 400 | 已经点赞过 / 文章不存在 |

---

#### 5.5.2 取消点赞（需认证）

```
DELETE /api/v1/articles/:id/like
```

---

#### 5.5.3 获取点赞信息（可选认证）

```
GET /api/v1/articles/:id/likes
```

**说明**: 未登录时 `is_liked` 始终为 `false`；登录后返回真实状态。

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "count": 10,
    "is_liked": true
  }
}
```

---

#### 5.5.4 发表评论（需认证）

```
POST /api/v1/articles/:id/comments
```

**请求体**:

```json
{
  "content": "这是一条评论",
  "parent_id": null
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content | string | ✅ | 评论内容 |
| parent_id | int | 否 | 父评论ID（回复时填写） |

**成功响应** `201 Created`:

```json
{
  "code": 201,
  "message": "created",
  "data": {
    "id": 1,
    "content": "这是一条评论",
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "avatar": "",
      "bio": "",
      "role": "user"
    },
    "article_id": 5,
    "parent_id": null,
    "replies": [],
    "created_at": "2025-07-12T10:00:00Z"
  }
}
```

---

#### 5.5.5 获取评论列表（公开）

```
GET /api/v1/articles/:id/comments?page=1&limit=10
```

**说明**: 返回顶级评论，每条评论包含嵌套回复（最多2层）。

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "content": "顶级评论",
      "user": { ... },
      "article_id": 5,
      "parent_id": null,
      "replies": [
        {
          "id": 2,
          "content": "回复评论",
          "user": { ... },
          "article_id": 5,
          "parent_id": 1,
          "replies": [],
          "created_at": "2025-07-12T11:00:00Z"
        }
      ],
      "created_at": "2025-07-12T10:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 1,
    "pages": 1
  }
}
```

---

#### 5.5.6 删除评论（需认证，仅评论者）

```
DELETE /api/v1/comments/:id
```

---

#### 5.5.7 收藏文章（需认证）

```
POST /api/v1/articles/:id/favorite
```

---

#### 5.5.8 取消收藏（需认证）

```
DELETE /api/v1/articles/:id/favorite
```

---

#### 5.5.9 检查是否已收藏（需认证）

```
GET /api/v1/articles/:id/favorite
```

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "is_favorited": true
  }
}
```

---

### 5.6 管理员接口

> 以下接口需认证且角色为 admin

#### 5.6.1 获取用户列表

```
GET /api/v1/admin/users?page=1&limit=10
```

**成功响应** `200 OK`:

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@blog.com",
      "avatar": "",
      "bio": "",
      "role": "admin"
    },
    {
      "id": 2,
      "username": "testuser",
      "email": "test@example.com",
      "avatar": "",
      "bio": "",
      "role": "user"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 2,
    "pages": 1
  }
}
```

---

## 6. 配置说明

### 6.1 配置文件

配置文件位于 `internal/config/config.yml`：

```yaml
# 应用配置
app:
  name: blog-system        # 应用名称
  port: ":8080"            # 监听端口
  mode: debug              # 运行模式：debug / release / test

# 数据库配置
database:
  driver: sqlite           # 数据库驱动：sqlite / mysql
  dsn: blog.db             # 连接字符串

# Redis配置（可选）
redis:
  host: localhost          # 主机
  port: "6379"             # 端口
  password: ""             # 密码
  db: 0                    # 数据库编号

# JWT配置
jwt:
  secret: your-secret-key  # 密钥（生产环境务必修改）
  expiration: 24           # Token有效期（小时）
```

### 6.2 环境变量覆盖

Viper 支持环境变量覆盖配置，格式为 `BLOG_SYSTEM_` 前缀 + 大写路径：

```bash
# 覆盖 app.port
export BLOG_SYSTEM_APP_PORT=":9090"

# 覆盖 jwt.secret
export BLOG_SYSTEM_JWT_SECRET="my-production-secret"
```

### 6.3 生产环境配置示例

```yaml
app:
  name: blog-system
  port: ":8080"
  mode: release

database:
  driver: mysql
  dsn: "user:password@tcp(127.0.0.1:3306)/blog_db?charset=utf8mb4&parseTime=True&loc=Local"

redis:
  host: 127.0.0.1
  port: "6379"
  password: "redis-password"
  db: 0

jwt:
  secret: "your-very-strong-secret-key-here"
  expiration: 72
```

---

## 7. 开发指南

### 7.1 添加新功能的标准流程

以「添加文章标签管理」为例：

**Step 1: Model** — `internal/model/tag.go`

```go
type Tag struct {
    ID   uint   `gorm:"primaryKey" json:"id"`
    Name string `gorm:"size:50;uniqueIndex;not null" json:"name"`
}
```

**Step 2: Repository** — `internal/repository/tag_repo.go`

```go
type TagRepository struct{}

func (r *TagRepository) FindOrCreate(tag *model.Tag) error {
    return database.DB.Where("name = ?", tag.Name).FirstOrCreate(tag).Error
}
```

**Step 3: Service** — `internal/service/tag_service.go`

```go
type TagService struct {
    tagRepo *repository.TagRepository
}

func (s *TagService) GetAll() ([]model.Tag, error) {
    // 业务逻辑
}
```

**Step 4: Handler** — `internal/handler/tag_handler.go`

```go
type TagHandler struct {
    tagService *service.TagService
}

func (h *TagHandler) GetAll(c *gin.Context) {
    tags, err := h.tagService.GetAll()
    if err != nil {
        response.InternalError(c, err.Error())
        return
    }
    response.Success(c, tags)
}
```

**Step 5: Router** — 在 `internal/router/router.go` 中注册路由

```go
tagHandler := handler.NewTagHandler()
api.GET("/tags", tagHandler.GetAll)
```

**Step 6: Migration** — 在 `cmd/server/main.go` 中添加自动迁移

```go
database.AutoMigrate(&model.Tag{})
```

### 7.2 错误处理规范

```go
// ✅ 正确：在 Service 层返回业务错误
func (s *ArticleService) GetByID(id uint) (*ArticleResponse, error) {
    article, err := s.articleRepo.FindByID(id)
    if err != nil {
        return nil, errors.New("文章不存在")
    }
    return toArticleResponse(article), nil
}

// ✅ 正确：在 Handler 层映射为 HTTP 状态码
func (h *ArticleHandler) GetByID(c *gin.Context) {
    article, err := h.articleService.GetByID(id)
    if err != nil {
        response.NotFound(c, err.Error())
        return
    }
    response.Success(c, article)
}

// ❌ 错误：在 Service 层直接操作 HTTP 响应
func (s *ArticleService) GetByID(id uint, c *gin.Context) {
    // 不要这样做
}
```

### 7.3 缓存使用规范

```go
// 读操作：先查缓存，再查数据库
func (s *ArticleService) GetByID(id uint) (*ArticleResponse, error) {
    cacheKey := fmt.Sprintf("article:%d", id)

    // 1. 尝试从缓存获取
    if cached, err := database.CacheGet(cacheKey); err == nil {
        var resp ArticleResponse
        json.Unmarshal([]byte(cached), &resp)
        return &resp, nil
    }

    // 2. 缓存未命中，查数据库
    article, err := s.articleRepo.FindByID(id)
    if err != nil {
        return nil, err
    }

    resp := toArticleResponse(article)

    // 3. 写入缓存
    if data, err := json.Marshal(resp); err == nil {
        database.CacheSet(cacheKey, string(data), 10*time.Minute)
    }

    return resp, nil
}

// 写操作：先更新数据库，再删除缓存
func (s *ArticleService) Update(...) {
    // 1. 更新数据库
    s.articleRepo.Update(article)

    // 2. 删除缓存（而非更新）
    database.CacheDelete(fmt.Sprintf("article:%d", id))
    database.CacheDelete("articles")
}
```

### 7.4 Makefile 命令

| 命令 | 说明 |
|------|------|
| `make build` | 编译项目到 `build/` 目录 |
| `make run` | 直接运行 |
| `make test` | 运行所有测试 |
| `make clean` | 清理构建文件和数据库 |
| `make deps` | 下载依赖 |
| `make tidy` | 整理依赖 |
| `make fmt` | 格式化代码 |
| `make vet` | 代码静态检查 |
| `make help` | 显示帮助 |

---

## 8. 部署指南

### 8.1 编译

```bash
# 交叉编译 Linux 版本
GOOS=linux GOARCH=amd64 go build -o blog-system ./cmd/server

# 编译当前平台版本
make build
```

### 8.2 目录结构（部署）

```
/opt/blog-system/
├── blog-system              # 可执行文件
├── config/
│   └── config.yml           # 配置文件
└── data/
    └── blog.db              # SQLite 数据库文件（开发模式）
```

### 8.3 Systemd 服务

```ini
# /etc/systemd/system/blog-system.service
[Unit]
Description=Blog System API Server
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/blog-system
ExecStart=/opt/blog-system/blog-system
Restart=always
RestartSec=5
Environment=BLOG_SYSTEM_APP_MODE=release
Environment=BLOG_SYSTEM_JWT_SECRET=your-production-secret

[Install]
WantedBy=multi-user.target
```

```bash
# 启动服务
sudo systemctl enable blog-system
sudo systemctl start blog-system

# 查看日志
sudo journalctl -u blog-system -f
```

### 8.4 Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o blog-system ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /app
COPY --from=builder /app/blog-system .
COPY --from=builder /app/internal/config/config.yml ./config/
EXPOSE 8080
CMD ["./blog-system"]
```

```bash
# 构建镜像
docker build -t blog-system .

# 运行容器
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e BLOG_SYSTEM_JWT_SECRET=your-secret \
  --name blog-system \
  blog-system
```

---

## 9. 常见问题

### 9.1 启动失败

**问题**: `连接数据库失败`

**解决**: 检查 `config.yml` 中的数据库配置。SQLite 模式下确保目录有写权限。

---

### 9.2 Token 过期

**问题**: 接口返回 `401 无效的token`

**解决**: 重新登录获取新 Token。默认有效期 24 小时，可在配置中修改 `jwt.expiration`。

---

### 9.3 Redis 连接失败

**问题**: 日志显示 `Redis连接失败，使用内存缓存`

**说明**: 这是正常行为。系统会自动降级到内存缓存，功能不受影响，但缓存不会在重启后保留。

---

### 9.4 跨域问题

**问题**: 前端请求被 CORS 策略阻止

**解决**: 在 `internal/middleware/cors.go` 中添加你的前端域名：

```go
AllowOrigins: []string{
    "http://localhost:5173",
    "http://localhost:3000",
    "https://your-domain.com",  // 添加这里
},
```

---

### 9.5 数据库迁移

**问题**: 新增字段后旧数据报错

**解决**: GORM 的 `AutoMigrate` 会自动添加新字段。如需手动迁移：

```bash
# SQLite
sqlite3 blog.db "ALTER TABLE articles ADD COLUMN cover_image VARCHAR(255);"

# MySQL
mysql -u root -p blog_db -e "ALTER TABLE articles ADD COLUMN cover_image VARCHAR(255);"
```

---

## 附录

### A. 完整 API 速查表

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /health | ❌ | 健康检查 |
| POST | /api/v1/auth/register | ❌ | 注册 |
| POST | /api/v1/auth/login | ❌ | 登录 |
| GET | /api/v1/user/profile | ✅ | 个人信息 |
| PUT | /api/v1/user/profile | ✅ | 更新个人信息 |
| PUT | /api/v1/user/password | ✅ | 修改密码 |
| GET | /api/v1/user/favorites | ✅ | 收藏列表 |
| GET | /api/v1/articles | ❌ | 文章列表 |
| GET | /api/v1/articles/search | ❌ | 搜索文章 |
| GET | /api/v1/articles/:id | ❌ | 文章详情 |
| POST | /api/v1/articles | ✅ | 创建文章 |
| PUT | /api/v1/articles/:id | ✅ | 更新文章 |
| DELETE | /api/v1/articles/:id | ✅ | 删除文章 |
| POST | /api/v1/articles/:id/like | ✅ | 点赞 |
| DELETE | /api/v1/articles/:id/like | ✅ | 取消点赞 |
| GET | /api/v1/articles/:id/likes | ❌ | 点赞信息 |
| POST | /api/v1/articles/:id/comments | ✅ | 发表评论 |
| GET | /api/v1/articles/:id/comments | ❌ | 评论列表 |
| DELETE | /api/v1/comments/:id | ✅ | 删除评论 |
| POST | /api/v1/articles/:id/favorite | ✅ | 收藏 |
| DELETE | /api/v1/articles/:id/favorite | ✅ | 取消收藏 |
| GET | /api/v1/articles/:id/favorite | ✅ | 是否收藏 |
| GET | /api/v1/admin/users | 🔒 | 用户列表（管理员） |

> ✅ = 需要认证 | ❌ = 无需认证 | 🔒 = 需要管理员权限

---

> 文档结束 | Blog System v1.0.0
