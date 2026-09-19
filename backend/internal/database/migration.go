package database

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"blog-system/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenMigrationDatabase 复用现有配置类型和驱动，既不建表也不初始化业务服务。
// 独立连接关闭 SQL 日志，防止 INSERT 参数或数据库错误携带文章内容。
func OpenMigrationDatabase(cfg config.Database, apply bool) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.Driver {
	case "sqlite":
		dsn, err := existingSQLiteDSN(cfg.DSN, apply)
		if err != nil {
			return nil, err
		}
		dialector = sqlite.Open(dsn)
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	default:
		return nil, errors.New("迁移仅支持 sqlite 或 mysql 驱动")
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("连接迁移数据库失败，请检查显式配置与网络；连接详情未输出")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.New("获取迁移连接失败")
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db, nil
}

// SQLite 明确只打开已有文件，防止路径错误时创建空库；dry-run 使用只读连接。
func existingSQLiteDSN(dsn string, apply bool) (string, error) {
	parts := strings.SplitN(dsn, "?", 2)
	name := parts[0]
	if strings.HasPrefix(name, "file:") {
		var err error
		name, err = url.PathUnescape(strings.TrimPrefix(name, "file:"))
		if err != nil {
			return "", errors.New("SQLite 文件路径无效")
		}
		// file:///D:/... 与普通 D:/... 在 Windows 指向同一个本地文件。
		if filepath.Separator == '\\' {
			local := strings.TrimLeft(name, "/")
			if len(local) >= 2 && local[1] == ':' {
				name = local
			}
		}
	}
	params := url.Values{}
	if len(parts) == 2 {
		var err error
		params, err = url.ParseQuery(parts[1])
		if err != nil {
			return "", errors.New("SQLite 参数无效")
		}
	}
	if name == "" || name == ":memory:" || params.Get("mode") == "memory" {
		return "", errors.New("迁移需要已有 SQLite 文件")
	}
	path, err := filepath.Abs(name)
	if err != nil {
		return "", errors.New("SQLite 文件路径无效")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("SQLite 文件不存在或不是普通文件；未创建新数据库")
	}
	params.Set("mode", "ro")
	if apply {
		params.Set("mode", "rw")
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath, RawQuery: params.Encode()}
	return uri.String(), nil
}
