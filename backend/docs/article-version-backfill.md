# 历史文章版本回填

旧 `articles` 数据升级结构后有 `version`、`published_version`，但 AutoMigrate 不会生成历史快照。
公开查询要求 `status=published`、`published_version>0` 且对应快照存在，因此这类旧文章会在首页消失。
此命令将已确认的旧文章当前内容保存为一份合成快照，不恢复不可考证的修改历史。

## 前提与范围

- 先备份数据库并验证备份可用；执行时停止所有后端实例及其他文章写入来源，仅运行一个迁移进程。
- 确认零快照记录确实来自版本机制引入前。快照被意外删除的数据不能仅凭零行就证明是旧数据，应先调查或恢复备份。
- 现有 Article、ArticleVersion 表及必需字段、组合唯一索引必须已由独立结构迁移建立。
- 必须显式设置项目已有的 `BLOG_DATABASE_DRIVER` 和 `BLOG_DATABASE_DSN`，不读取 `.env` 文件，也不回退到 config.yml/default SQLite。
- 不执行 AutoMigrate、不启动 HTTP/Redis、不创建管理员、不调用 Python、模型、RAG 或业务发布接口。
- `--dry-run` 或无参数只检查；只有 `--apply` 写入。二者不能同时使用。
- SQLite 要求已有文件；路径错误不创建空库。dry-run 使用只读文件连接。
- MySQL/TiDB 使用既有 GORM mysql 驱动。dry-run 无 DML/DDL，可配合只读数据库账号进一步限制权限。

## 迁移规则

扫描包括软删除记录在内的 Article ID，按 ID 分批读取；软删除记录单独统计并跳过。
每篇文章在独立事务内重新读取，apply 时使用 GORM 行锁声明，并重新检查版本集合。
停写和单进程是本命令的运行前提，不承诺不停机的在线迁移。

对于未删除、作者 ID 为正数、当前版本 N>0、没有任何 ArticleVersion 且公开指针为 0 的文章：

| 生命周期 | 快照 | 公开指针变更 |
| --- | --- | --- |
| draft | 创建 N | 保持 0 |
| published | 创建 N | 0 → N |
| archived | 创建 N | 保持 0，不重新公开 |

快照复制 Title、Content、Summary、CoverImage；CreatedBy 使用原 UserID，Source 使用既有 `user` 常量。
这只是兼容性归因，不声称能够还原历史修改人的身份。CreatedAt 使用原 UpdatedAt，零值时使用 CreatedAt，
两者都为零时使用本次命令开始扫描的时间。dry-run 与 apply 分次执行时，最后这一回退时间可以不同。

Article.Version 原值保持不变，N>1 不补造 1..N-1。只更新需要修复的 PublishedVersion，
使用 UpdateColumn 避免更改 Article.UpdatedAt。原正文、标题、标签、作者、计数和生命周期均不修改。
快照插入与指针修复在同一事务中，任一写入或提交失败均不会留下半迁移状态。

已有版本且当前/公开指针有效的文章跳过。以下情况报告为 inconsistent，原样保留：

- Version=0、UserID=0、未知生命周期。
- 已有版本但当前版本快照缺失，或存在版本 0/大于当前版本的快照。
- 非零 PublishedVersion 找不到快照，包括零快照文章；不会擅自重指向当前正文。
- 已有快照且 status=published，但 PublishedVersion=0。
- draft 却带非零 PublishedVersion。

归档保留已有有效公开指针，与现有 Archive 语义一致；不会统一清零。
快照集合已存在但不完整的数据不会与真正旧数据混用一套修复规则。

## 输出、幂等与退出码

输出逐篇 JSON：article_id、outcome、reason、version_no、published_before/after、snapshot_at、legacy。
不输出标题、正文、DSN、密码、JWT 或底层数据库错误原文；迁移连接关闭 GORM SQL 日志。

最后输出 inspected、legacy_zero_versions、would_migrate、migrated、skipped、already_versioned、
deleted_skipped、inconsistent、failed。软删除不计入 legacy_zero_versions；统计均针对本次实际扫描结果。
would_migrate 仅用于 dry-run，实际执行成功计入 migrated。

- 退出码 0：本次扫描没有异常或失败。
- 退出码 1：连接/结构/扫描失败，或存在 inconsistent/failed；逐篇事务意味着其他文章可能已成功提交。
- 退出码 2：参数或显式配置缺失；不连接数据库。

重跑会明确识别已存在版本并跳过，不递增 Article.Version；组合唯一索引是最后防线。
单篇失败会继续处理其余记录并最终返回非零，修正原因后可重跑。不要用重复执行来忽略异常。
命令验证模型所用索引名及唯一性、列顺序。旧版 GORM SQLite 驱动不支持 GetIndexes，
仅其结构检查使用只读 PRAGMA；MySQL/TiDB 使用 GORM GetIndexes，实际回填逻辑完全相同。

## 本地 SQLite

在 `backend` 目录执行，明确指定真实库，先查看计划：

```powershell
$env:BLOG_DATABASE_DRIVER = 'sqlite'
$env:BLOG_DATABASE_DSN = (Resolve-Path .\blog.db).Path
go run ./cmd/backfill-article-versions --dry-run
```

确认停写后，通过 SQLite 备份工具或 SQLite backup API 保存一致备份。
不要在仍有写入或未处理 WAL 时只复制单个 .db 文件。备份、日志必须放在 Git 忽略目录。
审核计划和异常后执行：

```powershell
go run ./cmd/backfill-article-versions --apply
go run ./cmd/backfill-article-versions --dry-run
```

第二次检查应显示 would_migrate=0；本次迁移的文章变为 already_versioned。
随后启动 Go、刷新首页，并登录原作者账号检查历史版本与编辑页。

## 生产 MySQL / TiDB

1. 先在非生产的同类数据库上验证命令；普通 SQLite 单元测试不能替代 MySQL/TiDB 集成验证。
2. 安排维护窗口、备份/快照数据库，停止全部文章写入实例；部署包含该命令的代码/可执行文件。
3. 通过部署环境的秘密配置注入 `BLOG_DATABASE_DRIVER=mysql` 和 `BLOG_DATABASE_DSN`，使用既有 MySQL/TiDB DSN（含 TLS 要求）。不要将凭据写进仓库或操作记录。
4. 确认 schema 和索引已迁移，运行 dry-run；审核数据库目标、逐篇版本身份、指针调整、总数及异常。异常必须先调查，尤其是曾运行过新版本但可能丢失快照的数据。
5. 保持停写、单进程执行 apply。报告为非零时检查部分成功结果，不把整批当作未执行。
6. 再次 dry-run，确认没有待迁移记录，异常有明确处理结论。
7. 恢复后端，验证公开列表/详情、作者版本历史和工作正文。现有公开缓存会先对照数据库记录及版本指针；本命令不清空共享 Redis。
8. 用专用测试文章验证 N → 手动保存 N+1 → Diff → Agent 提案 → Preview → 人工 Apply N+2，确认公开指针仍按业务发布操作推进。不要为冒烟测试随意修改真实生产文章。

构建命令（在 backend 目录）：

```sh
go build -o backfill-article-versions ./cmd/backfill-article-versions
./backfill-article-versions --dry-run
./backfill-article-versions --apply
./backfill-article-versions --dry-run
```

TiDB 事务模式由部署配置决定，参见 [TiDB 官方事务说明](https://docs.pingcap.com/tidb/stable/transaction-overview/)。
本工具不修改生产事务设置，不宣称在所有模式下支持并发迁移；维护窗口仍是必要前提。
迁移失败优先排查并幂等重跑；需要恢复备份时必须再次停写，并评估备份后新写入，不能直接覆盖在线数据库。

## 验证

```sh
go test ./...
```

测试覆盖状态/非 V1 映射、幂等、时间回退、dry-run 无写入、只读文件连接、缺失结构和伪装的非唯一索引、
部分历史异常、归档指针、软删除、两处写入失败回滚及恢复、部分成功后重跑。
服务集成测试使用隔离 SQLite，验证回填后仍可沿用原来的列表、版本历史、保存、Diff、Preview 和 Apply。
