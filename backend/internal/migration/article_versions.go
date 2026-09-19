// Package migration 提供显式执行的数据修复，不参与应用启动或业务请求。
package migration

import (
	"context"
	"errors"
	"time"

	"blog-system/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var ErrIncomplete = errors.New("迁移存在异常或失败，请检查逐篇结果；已提交的文章不会回滚")

// Result 仅包含审计所需的身份和元数据，不包含正文、标题或数据库错误原文。
type Result struct {
	ArticleID       uint      `json:"article_id"`
	Outcome         string    `json:"outcome"`
	Reason          string    `json:"reason,omitempty"`
	VersionNo       uint      `json:"version_no"`
	PublishedBefore uint      `json:"published_before"`
	PublishedAfter  uint      `json:"published_after"`
	SnapshotAt      time.Time `json:"snapshot_at,omitempty"`
	Legacy          bool      `json:"legacy"`
}

type Report struct {
	Inspected        int `json:"inspected"`
	Legacy           int `json:"legacy_zero_versions"`
	WouldMigrate     int `json:"would_migrate"`
	Migrated         int `json:"migrated"`
	Skipped          int `json:"skipped"`
	AlreadyVersioned int `json:"already_versioned"`
	Deleted          int `json:"deleted_skipped"`
	Inconsistent     int `json:"inconsistent"`
	Failed           int `json:"failed"`
}

// CheckSchema 只检查既有结构，绝不执行 AutoMigrate 或修复 DDL。
func CheckSchema(db *gorm.DB) error {
	for _, item := range []struct {
		value   interface{}
		columns []string
	}{
		{&model.Article{}, []string{"id", "version", "published_version", "title", "content", "summary", "cover_image", "user_id", "status", "created_at", "updated_at", "deleted_at"}},
		{&model.ArticleVersion{}, []string{"id", "article_id", "version_no", "title", "content", "summary", "cover_image", "created_by", "source", "created_at"}},
	} {
		if !db.Migrator().HasTable(item.value) {
			return errors.New("缺少文章或版本表，请先完成独立的结构迁移")
		}
		for _, column := range item.columns {
			if !db.Migrator().HasColumn(item.value, column) {
				return errors.New("文章或版本表缺少必需列，请先完成独立的结构迁移")
			}
		}
	}
	if !hasVersionUniqueIndex(db) {
		return errors.New("缺少文章版本组合唯一索引，请先完成独立的结构迁移")
	}
	return nil
}

// 当前 SQLite GORM 驱动没有实现 GetIndexes，仅元数据检查需要方言适配。
// 实际回填、判定与事务逻辑对 SQLite 和 MySQL/TiDB 完全共用。
func hasVersionUniqueIndex(db *gorm.DB) bool {
	const name = "idx_article_versions_article_version"
	if db.Dialector.Name() == "sqlite" {
		var indexes []struct {
			Name    string
			Unique  int
			Partial int
		}
		if err := db.Raw("PRAGMA index_list('article_versions')").Scan(&indexes).Error; err != nil {
			return false
		}
		for _, index := range indexes {
			if index.Name != name || index.Unique != 1 || index.Partial != 0 {
				continue
			}
			var columns []struct{ Name string }
			if err := db.Raw("PRAGMA index_info('idx_article_versions_article_version')").Scan(&columns).Error; err != nil {
				return false
			}
			return len(columns) == 2 && columns[0].Name == "article_id" && columns[1].Name == "version_no"
		}
		return false
	}
	indexes, err := db.Migrator().GetIndexes(&model.ArticleVersion{})
	if err != nil {
		return false
	}
	for _, index := range indexes {
		unique, known := index.Unique()
		columns := index.Columns()
		if index.Name() == name && known && unique && len(columns) == 2 && columns[0] == "article_id" && columns[1] == "version_no" {
			return true
		}
	}
	return false
}

// Run 默认调用方只检查；apply=true 必须在停写维护窗口中由单个进程执行。
// 每篇文章独立提交，失败后可安全重跑；不会因一次失败回滚之前的成功文章。
func Run(ctx context.Context, db *gorm.DB, apply bool, emit func(Result)) (Report, error) {
	db = db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	var report Report
	if err := CheckSchema(db); err != nil {
		return report, err
	}
	now := time.Now().UTC()
	var lastID uint
	for {
		var ids []uint
		if err := db.Unscoped().Model(&model.Article{}).Where("id > ?", lastID).Order("id").Limit(100).Pluck("id", &ids).Error; err != nil {
			return report, errors.New("读取文章批次失败，未输出底层错误以避免泄漏数据")
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			result := Result{ArticleID: id}
			// 重新读取和分类均在同一事务中；不把 dry-run 结果当作写入授权或快照。
			err := db.Transaction(func(tx *gorm.DB) error {
				var article model.Article
				query := tx.Unscoped()
				if apply {
					query = query.Clauses(clause.Locking{Strength: "UPDATE"})
				}
				if err := query.First(&article, id).Error; err != nil {
					return err
				}
				var err error
				result, err = inspect(tx, article, now)
				if err != nil || !apply || result.Outcome != "candidate" {
					return err
				}
				snapshot := model.ArticleVersion{
					ArticleID: article.ID, VersionNo: article.Version, Title: article.Title,
					Content: article.Content, Summary: article.Summary, CoverImage: article.CoverImage,
					CreatedBy: article.UserID, Source: model.ArticleVersionSourceUser, CreatedAt: result.SnapshotAt,
				}
				if err := tx.Create(&snapshot).Error; err != nil {
					return err
				}
				if result.PublishedAfter != article.PublishedVersion {
					// 只修复指针，不触发 UpdatedAt，也不保存整行或任何关联关系。
					updated := tx.Model(&model.Article{}).Where("id = ? AND version = ? AND published_version = ? AND status = ?",
						article.ID, article.Version, article.PublishedVersion, article.Status).
						UpdateColumn("published_version", result.PublishedAfter)
					if updated.Error != nil {
						return updated.Error
					}
					if updated.RowsAffected != 1 {
						return errors.New("文章状态已变化")
					}
				}
				return nil
			})
			if err != nil {
				result.Outcome, result.Reason = "failed", "读取、写入或事务提交失败；请保持停写后排查，正文及底层错误未输出"
			} else if apply && result.Outcome == "candidate" {
				result.Outcome = "migrated"
			}
			report.Inspected++
			if result.Legacy {
				report.Legacy++
			}
			switch result.Outcome {
			case "candidate":
				report.WouldMigrate++
			case "migrated":
				report.Migrated++
			case "already_versioned":
				report.Skipped++
				report.AlreadyVersioned++
			case "deleted":
				report.Skipped++
				report.Deleted++
			case "inconsistent":
				report.Inconsistent++
			case "failed":
				report.Failed++
			}
			if emit != nil {
				emit(result)
			}
			lastID = id
		}
	}
	if report.Inconsistent > 0 || report.Failed > 0 {
		return report, ErrIncomplete
	}
	return report, nil
}

func inspect(tx *gorm.DB, article model.Article, now time.Time) (Result, error) {
	r := Result{ArticleID: article.ID, VersionNo: article.Version,
		PublishedBefore: article.PublishedVersion, PublishedAfter: article.PublishedVersion}
	if article.DeletedAt.Valid {
		r.Outcome = "deleted"
		return r, nil
	}
	var numbers []uint
	if err := tx.Model(&model.ArticleVersion{}).Where("article_id = ?", article.ID).Pluck("version_no", &numbers).Error; err != nil {
		return r, err
	}
	r.Legacy = len(numbers) == 0
	invalid := func(reason string) (Result, error) { r.Outcome, r.Reason = "inconsistent", reason; return r, nil }
	if article.Version == 0 {
		return invalid("current_version_not_positive")
	}
	if article.UserID == 0 {
		return invalid("author_id_not_positive")
	}
	if article.Status != model.ArticleStatusDraft && article.Status != model.ArticleStatusPublished && article.Status != model.ArticleStatusArchived {
		return invalid("unknown_lifecycle_status")
	}
	hasCurrent, hasPublished := false, false
	for _, n := range numbers {
		if n == 0 || n > article.Version {
			return invalid("snapshot_version_out_of_range")
		}
		hasCurrent = hasCurrent || n == article.Version
		hasPublished = hasPublished || n == article.PublishedVersion
	}
	// 非零悬空公开指针优先判为异常，即使整篇文章完全没有快照也不推断历史。
	if article.PublishedVersion > 0 && !hasPublished {
		return invalid("published_snapshot_missing")
	}
	if article.Status == model.ArticleStatusDraft && article.PublishedVersion != 0 {
		return invalid("draft_has_published_pointer")
	}
	if !r.Legacy {
		if !hasCurrent {
			return invalid("current_snapshot_missing")
		}
		if article.Status == model.ArticleStatusPublished && article.PublishedVersion == 0 {
			return invalid("published_pointer_missing")
		}
		r.Outcome = "already_versioned"
		return r, nil
	}
	r.Outcome = "candidate"
	r.SnapshotAt = article.UpdatedAt
	if r.SnapshotAt.IsZero() {
		r.SnapshotAt = article.CreatedAt
	}
	if r.SnapshotAt.IsZero() {
		r.SnapshotAt = now
	}
	if article.Status == model.ArticleStatusPublished {
		r.PublishedAfter = article.Version
	}
	return r, nil
}
