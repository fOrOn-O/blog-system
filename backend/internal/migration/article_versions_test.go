package migration_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/migration"
	"blog-system/internal/model"
	"blog-system/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setup(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Article{}, &model.ArticleVersion{}, &model.Tag{}); err != nil {
		t.Fatal(err)
	}
	user := model.User{ID: 1, Username: "legacy", Email: "legacy@example.test", Password: "fixture"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

func legacy(t *testing.T, db *gorm.DB, status string, version uint) model.Article {
	t.Helper()
	a := model.Article{UserID: 1, Version: version, Title: "旧文章", Content: "<p>旧正文</p>", Summary: "摘要", CoverImage: "/cover.png", Status: status,
		ViewCount: 8, LikeCount: 3, CommentCount: 2, CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	return a
}

func article(t *testing.T, db *gorm.DB, id uint) model.Article {
	t.Helper()
	var a model.Article
	if err := db.Unscoped().First(&a, id).Error; err != nil {
		t.Fatal(err)
	}
	return a
}

func snapshots(t *testing.T, db *gorm.DB) []model.ArticleVersion {
	t.Helper()
	var rows []model.ArticleVersion
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func execute(t *testing.T, db *gorm.DB, apply bool) (migration.Report, []migration.Result, error) {
	t.Helper()
	var results []migration.Result
	report, err := migration.Run(context.Background(), db, apply, func(r migration.Result) { results = append(results, r) })
	return report, results, err
}

func TestLegacyStatusesPreserveVersionAndBusinessData(t *testing.T) {
	for _, status := range []string{model.ArticleStatusDraft, model.ArticleStatusPublished, model.ArticleStatusArchived} {
		for _, version := range []uint{1, 7} {
			t.Run(status+time.Duration(version).String(), func(t *testing.T) {
				db := setup(t)
				a := legacy(t, db, status, version)
				tag := model.Tag{Name: "原标签"}
				if err := db.Create(&tag).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Model(&a).Association("Tags").Append(&tag); err != nil {
					t.Fatal(err)
				}
				before := article(t, db, a.ID)
				report, _, err := execute(t, db, true)
				if err != nil || report.Migrated != 1 || report.Legacy != 1 {
					t.Fatalf("%+v %v", report, err)
				}
				after := article(t, db, a.ID)
				want := before
				if status == model.ArticleStatusPublished {
					want.PublishedVersion = version
				}
				if !reflect.DeepEqual(want, after) {
					t.Fatal("迁移改变了非目标文章字段或时间")
				}
				if n := db.Model(&a).Association("Tags").Count(); n != 1 {
					t.Fatal("标签关系改变")
				}
				rows := snapshots(t, db)
				if len(rows) != 1 {
					t.Fatal("快照数量错误")
				}
				v := rows[0]
				if v.ArticleID != a.ID || v.VersionNo != version || v.Title != a.Title || v.Content != a.Content || v.Summary != a.Summary || v.CoverImage != a.CoverImage || v.CreatedBy != a.UserID || v.Source != model.ArticleVersionSourceUser || !v.CreatedAt.Equal(before.UpdatedAt) {
					t.Fatal("快照不匹配")
				}
				report, _, err = execute(t, db, true)
				if err != nil || report.AlreadyVersioned != 1 || report.Migrated != 0 || !reflect.DeepEqual(rows, snapshots(t, db)) || !reflect.DeepEqual(after, article(t, db, a.ID)) {
					t.Fatal("重跑不幂等")
				}
			})
		}
	}
}

func TestDryRunAndTimestampFallbacks(t *testing.T) {
	db := setup(t)
	updated := legacy(t, db, model.ArticleStatusPublished, 3)
	created := legacy(t, db, model.ArticleStatusDraft, 4)
	missing := legacy(t, db, model.ArticleStatusArchived, 5)
	if err := db.Model(&created).UpdateColumn("updated_at", time.Time{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&missing).UpdateColumns(map[string]interface{}{"created_at": time.Time{}, "updated_at": time.Time{}}).Error; err != nil {
		t.Fatal(err)
	}
	var before, after []model.Article
	db.Order("id").Find(&before)
	started := time.Now().UTC()
	report, results, err := execute(t, db, false)
	if err != nil || report.WouldMigrate != 3 || report.Migrated != 0 {
		t.Fatalf("%+v %v", report, err)
	}
	db.Order("id").Find(&after)
	if !reflect.DeepEqual(before, after) || len(snapshots(t, db)) != 0 {
		t.Fatal("dry-run 写入了数据")
	}
	if !results[0].SnapshotAt.Equal(updated.UpdatedAt) || !results[1].SnapshotAt.Equal(created.CreatedAt) || results[2].SnapshotAt.Before(started) {
		t.Fatal("时间回退错误")
	}
	if results[0].PublishedBefore != 0 || results[0].PublishedAfter != 3 {
		t.Fatal("计划缺少公开指针修复")
	}
	if _, _, err := execute(t, db, true); err != nil {
		t.Fatal(err)
	}
	rows := snapshots(t, db)
	if !rows[0].CreatedAt.Equal(updated.UpdatedAt) || !rows[1].CreatedAt.Equal(created.CreatedAt) || rows[2].CreatedAt.Before(started) {
		t.Fatal("实际快照时间错误")
	}
}

func TestInconsistentRowsAreNotManufactured(t *testing.T) {
	cases := []struct {
		name, status, reason string
		current, published   uint
		versions             []uint
	}{
		{"missing_current", "published", "current_snapshot_missing", 3, 1, []uint{1}},
		{"missing_public", "published", "published_snapshot_missing", 3, 2, []uint{3}},
		{"zero_snapshots_dangling_public", "archived", "published_snapshot_missing", 3, 3, nil},
		{"published_zero_pointer", "published", "published_pointer_missing", 3, 0, []uint{3}},
		{"zero_current", "draft", "current_version_not_positive", 0, 0, nil},
		{"bad_status", "other", "unknown_lifecycle_status", 1, 0, nil},
		{"future_snapshot", "draft", "snapshot_version_out_of_range", 1, 0, []uint{2}},
		{"draft_public_pointer", "draft", "draft_has_published_pointer", 1, 1, []uint{1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := setup(t)
			a := legacy(t, db, c.status, 1)
			db.Model(&a).UpdateColumns(map[string]interface{}{"version": c.current, "published_version": c.published})
			for _, n := range c.versions {
				if err := db.Create(&model.ArticleVersion{ArticleID: a.ID, VersionNo: n, Title: a.Title, Content: a.Content, CreatedBy: 1, Source: model.ArticleVersionSourceUser}).Error; err != nil {
					t.Fatal(err)
				}
			}
			before, versions := article(t, db, a.ID), snapshots(t, db)
			for _, apply := range []bool{false, true} {
				report, results, err := execute(t, db, apply)
				if !errors.Is(err, migration.ErrIncomplete) || report.Inconsistent != 1 || results[0].Reason != c.reason {
					t.Fatalf("%+v %+v %v", report, results, err)
				}
				if !reflect.DeepEqual(before, article(t, db, a.ID)) || !reflect.DeepEqual(versions, snapshots(t, db)) {
					t.Fatal("异常文章被更改")
				}
			}
		})
	}
}

func TestArchivedPointerAndDeletedRowsRemainUntouched(t *testing.T) {
	db := setup(t)
	a := legacy(t, db, model.ArticleStatusArchived, 7)
	db.Model(&a).UpdateColumn("published_version", 6)
	for _, n := range []uint{6, 7} {
		db.Create(&model.ArticleVersion{ArticleID: a.ID, VersionNo: n, Title: a.Title, Content: a.Content, CreatedBy: 1, Source: model.ArticleVersionSourceUser})
	}
	deleted := legacy(t, db, model.ArticleStatusPublished, 1)
	db.Delete(&deleted)
	before, removed, versions := article(t, db, a.ID), article(t, db, deleted.ID), snapshots(t, db)
	r, _, err := execute(t, db, true)
	if err != nil || r.Inspected != 2 || r.AlreadyVersioned != 1 || r.Deleted != 1 || r.Skipped != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	if !reflect.DeepEqual(before, article(t, db, a.ID)) || !reflect.DeepEqual(removed, article(t, db, deleted.ID)) || !reflect.DeepEqual(versions, snapshots(t, db)) {
		t.Fatal("修改了归档指针或删除记录")
	}
}

func TestEveryWriteFailureRollsBackAndCanResume(t *testing.T) {
	for _, stage := range []string{"snapshot", "pointer"} {
		t.Run(stage, func(t *testing.T) {
			db := setup(t)
			a := legacy(t, db, model.ArticleStatusPublished, 7)
			before := article(t, db, a.ID)
			failure := func(tx *gorm.DB) { tx.AddError(errors.New("敏感正文不应进入报告")) }
			if stage == "snapshot" {
				db.Callback().Create().Before("gorm:create").Register("migration_failure", failure)
			} else {
				db.Callback().Update().Before("gorm:update").Register("migration_failure", failure)
			}
			r, results, err := execute(t, db, true)
			if !errors.Is(err, migration.ErrIncomplete) || r.Failed != 1 || r.Migrated != 0 || results[0].Outcome != "failed" {
				t.Fatalf("%+v %v", r, err)
			}
			if !reflect.DeepEqual(before, article(t, db, a.ID)) || len(snapshots(t, db)) != 0 {
				t.Fatal("存在半迁移状态")
			}
			db.Callback().Create().Remove("migration_failure")
			db.Callback().Update().Remove("migration_failure")
			if r, _, err = execute(t, db, true); err != nil || r.Migrated != 1 {
				t.Fatal("回滚后无法恢复")
			}
		})
	}
}

func TestSchemaPreflightDoesNotCreateMissingIndex(t *testing.T) {
	db := setup(t)
	db.Migrator().DropIndex(&model.ArticleVersion{}, "idx_article_versions_article_version")
	if _, _, err := execute(t, db, false); err == nil {
		t.Fatal("没有拒绝缺失唯一约束")
	}
	if db.Migrator().HasIndex(&model.ArticleVersion{}, "idx_article_versions_article_version") {
		t.Fatal("dry-run 创建了索引")
	}
	if err := db.Exec("CREATE INDEX idx_article_versions_article_version ON article_versions(article_id, version_no)").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, db, false); err == nil {
		t.Fatal("同名非唯一索引被误认为约束")
	}
}

func TestPartialProgressIsReportedAndRerunSkipsSuccess(t *testing.T) {
	db := setup(t)
	legacy(t, db, model.ArticleStatusPublished, 1)
	legacy(t, db, "invalid", 1)
	legacy(t, db, model.ArticleStatusDraft, 3)
	r, _, err := execute(t, db, true)
	if !errors.Is(err, migration.ErrIncomplete) || r.Inspected != 3 || r.Migrated != 2 || r.Inconsistent != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	before := snapshots(t, db)
	r, _, err = execute(t, db, true)
	if !errors.Is(err, migration.ErrIncomplete) || r.Migrated != 0 || r.AlreadyVersioned != 2 || !reflect.DeepEqual(before, snapshots(t, db)) {
		t.Fatal("部分成功后的重跑行为错误")
	}
}

func TestMigratedVersionContinuesThroughExistingServices(t *testing.T) {
	db := setup(t)
	previous := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		database.CacheDeletePrefix("article:")
		database.CacheDeletePrefix("articles:list:")
	})
	a := legacy(t, db, model.ArticleStatusPublished, 7)
	if _, _, err := execute(t, db, true); err != nil {
		t.Fatal(err)
	}
	svc := service.NewArticleService()
	listed, total, err := svc.List(1, 10, "")
	if err != nil || total != 1 || listed[0].Version != 7 {
		t.Fatal("公开列表无法读取回填快照")
	}
	history, total, err := svc.ListOwnedVersions(1, a.ID, 1, 20)
	if err != nil || total != 1 || history[0].VersionNo != 7 {
		t.Fatal("历史读取失败")
	}
	body := "<p>手动编辑后的正文</p>"
	saved, err := svc.UpdateDraft(1, a.ID, service.UpdateDraftRequest{ExpectedVersion: 7, Content: &body})
	if err != nil || saved.Version != 8 || saved.PublishedVersion != 7 {
		t.Fatal("手动保存未延续版本")
	}
	diff, err := svc.GetVersionDiff(1, a.ID, 7, 8)
	if err != nil || !diff.Content.Changed {
		t.Fatal("版本差异失败")
	}
	proposal := service.ArticleEditRequest{BaseVersionNo: 8, ProposedContent: "<p>人工批准的提案</p>"}
	preview, err := svc.PreviewArticleEdit(1, a.ID, proposal)
	if err != nil || !preview.Content.Changed {
		t.Fatal("预览失败")
	}
	result, err := svc.ApplyArticleEdit(1, a.ID, proposal)
	if err != nil || result.NewVersionNo != 9 || result.PublishedVersion != 7 {
		t.Fatal("应用未延续版本")
	}
	listed, _, err = svc.List(1, 10, "")
	if err != nil || listed[0].Content != a.Content || listed[0].Version != 7 {
		t.Fatal("提案修改泄漏到公开内容")
	}
}
