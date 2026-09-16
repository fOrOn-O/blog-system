package repository

import (
	"errors"

	"blog-system/internal/database"
	"blog-system/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ArticleRepository 文章数据访问层
type ArticleRepository struct{}

// NewArticleRepository 创建文章Repository
func NewArticleRepository() *ArticleRepository {
	return &ArticleRepository{}
}

// Create 创建文章并保存标签关联
func (r *ArticleRepository) Create(article *model.Article, tags []model.Tag) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		article.Version = 1
		article.PublishedVersion = 0
		if article.Status == model.ArticleStatusPublished {
			article.PublishedVersion = 1
		}
		if err := tx.Omit("Tags", "User").Create(article).Error; err != nil {
			return err
		}

		if len(tags) > 0 {
			if err := tx.Model(article).Association("Tags").Replace(tags); err != nil {
				return err
			}
		}

		return tx.Create(newArticleVersion(article, article.UserID)).Error
	})
}

// FindByID 根据ID查找文章
func (r *ArticleRepository) FindByID(id uint) (*model.Article, error) {
	var article model.Article
	err := database.DB.Preload("User").Preload("Tags").First(&article, id).Error
	return &article, err
}

// UpdateDraft 只保存工作内容，不推进发布状态或公开版本。
func (r *ArticleRepository) UpdateDraft(article *model.Article, tags []model.Tag, replaceTags bool, createdBy, expectedVersion uint) error {
	return r.updateContent(article, tags, replaceTags, createdBy, expectedVersion, false)
}

// UpdateAndPublish 将人工编辑、快照和公开版本在同一事务中提交。
func (r *ArticleRepository) UpdateAndPublish(article *model.Article, tags []model.Tag, replaceTags bool, createdBy, expectedVersion uint) error {
	return r.updateContent(article, tags, replaceTags, createdBy, expectedVersion, true)
}

func (r *ArticleRepository) updateContent(article *model.Article, tags []model.Tag, replaceTags bool, createdBy, expectedVersion uint, publish bool) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		current, err := findOwnedArticleForUpdate(tx, createdBy, article.ID)
		if err != nil {
			return err
		}
		if current.Status == model.ArticleStatusArchived {
			return model.ErrArticleArchivedEdit
		}
		// 必须先检查版本，再判断内容是否变化，不能放行过期的空更新或标签更新。
		if err := checkExpectedVersion(current, expectedVersion); err != nil {
			return err
		}
		contentChanged := current.Title != article.Title ||
			current.Content != article.Content ||
			current.Summary != article.Summary ||
			current.CoverImage != article.CoverImage
		article.Version = current.Version
		article.Status = current.Status
		article.PublishedVersion = current.PublishedVersion
		if contentChanged {
			article.Version++
		}
		if publish {
			if !contentChanged {
				if err := requireArticleSnapshot(tx, article.ID, article.Version); err != nil {
					return err
				}
			}
			article.Status = model.ArticleStatusPublished
			article.PublishedVersion = article.Version
		}

		if err := tx.Omit("Tags", "User").Save(article).Error; err != nil {
			return err
		}

		if replaceTags {
			if err := tx.Model(article).Association("Tags").Replace(tags); err != nil {
				return err
			}
		}

		if contentChanged {
			return tx.Create(newArticleVersion(article, createdBy)).Error
		}
		return nil
	})
}

func newArticleVersion(article *model.Article, createdBy uint) *model.ArticleVersion {
	return &model.ArticleVersion{
		ArticleID:  article.ID,
		VersionNo:  article.Version,
		Title:      article.Title,
		Content:    article.Content,
		Summary:    article.Summary,
		CoverImage: article.CoverImage,
		CreatedBy:  createdBy,
		Source:     model.ArticleVersionSourceUser,
	}
}

// Delete 删除文章（级联删除关联数据）
func (r *ArticleRepository) Delete(id uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		// 删除文章的点赞记录
		if err := tx.Where("article_id = ?", id).Delete(&model.Like{}).Error; err != nil {
			return err
		}

		// 删除文章的收藏记录
		if err := tx.Where("article_id = ?", id).Delete(&model.Favorite{}).Error; err != nil {
			return err
		}

		// 删除文章的评论
		if err := tx.Where("article_id = ?", id).Delete(&model.Comment{}).Error; err != nil {
			return err
		}

		// 删除文章的标签关联
		if err := tx.Exec("DELETE FROM article_tags WHERE article_id = ?", id).Error; err != nil {
			return err
		}

		// 最后删除文章本身
		if err := tx.Delete(&model.Article{}, id).Error; err != nil {
			return err
		}

		return nil
	})
}

// 公开读取仅使用已发布快照的内容，不回退到工作版本内容。
// TODO: 上线前补齐历史数据的版本快照和 published_version。
func publishedArticleQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&model.Article{}).
		Joins("JOIN article_versions AS published ON published.article_id = articles.id AND published.version_no = articles.published_version").
		Where("articles.status = ? AND articles.published_version > 0", model.ArticleStatusPublished)
}

const publishedArticleColumns = `articles.id, articles.user_id, articles.status,
	articles.published_version AS version, articles.published_version,
	published.title, published.content, published.summary, published.cover_image,
	articles.view_count, articles.like_count, articles.comment_count,
	articles.created_at, articles.updated_at, articles.deleted_at`

func (r *ArticleRepository) FindPublishedByID(id uint) (*model.Article, error) {
	var article model.Article
	err := publishedArticleQuery(database.DB).Select(publishedArticleColumns).
		Preload("User").Preload("Tags").First(&article, id).Error
	return &article, err
}

func (r *ArticleRepository) List(page, limit int) ([]model.Article, int64, error) {
	return listPublishedArticles(publishedArticleQuery(database.DB), page, limit)
}

func (r *ArticleRepository) Search(keyword string, page, limit int) ([]model.Article, int64, error) {
	query := publishedArticleQuery(database.DB).
		Where("(published.title LIKE ? OR published.content LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	return listPublishedArticles(query, page, limit)
}

func listPublishedArticles(query *gorm.DB, page, limit int) ([]model.Article, int64, error) {
	var articles []model.Article
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Select(publishedArticleColumns).Preload("User").Preload("Tags").
		Offset((page - 1) * limit).Limit(limit).
		Order("articles.created_at DESC, articles.id DESC").Find(&articles).Error
	return articles, total, err
}

func findOwnedArticle(db *gorm.DB, userID, articleID uint) (*model.Article, error) {
	var article model.Article
	if err := db.First(&article, articleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrArticleNotFound
		}
		return nil, err
	}
	if article.UserID != userID {
		return nil, model.ErrArticleForbidden
	}
	return &article, nil
}

// 仅用于写事务。MySQL/InnoDB 行锁保持到提交或回滚，覆盖版本检查及后续写入。
// SQLite 驱动省略 FOR UPDATE，使用数据库自身的事务锁，不能等同于 MySQL 行锁测试。
func findOwnedArticleForUpdate(tx *gorm.DB, userID, articleID uint) (*model.Article, error) {
	return findOwnedArticle(tx.Clauses(clause.Locking{Strength: "UPDATE"}), userID, articleID)
}

func checkExpectedVersion(article *model.Article, expectedVersion uint) error {
	if expectedVersion == 0 {
		return model.ErrExpectedVersionRequired
	}
	if article.Version != expectedVersion {
		return model.ErrVersionConflict
	}
	return nil
}

func (r *ArticleRepository) FindOwnedByID(userID, articleID uint) (*model.Article, error) {
	return findOwnedArticle(database.DB.Preload("User").Preload("Tags"), userID, articleID)
}

// FindOwnedVersions 先校验文章归属，再查询快照，不读取其他文章的历史版本。
func (r *ArticleRepository) FindOwnedVersions(userID, articleID, from, to uint) (*model.ArticleVersion, *model.ArticleVersion, error) {
	if _, err := findOwnedArticle(database.DB, userID, articleID); err != nil {
		return nil, nil, err
	}
	var versions []model.ArticleVersion
	err := database.DB.Where("article_id = ? AND version_no IN ?", articleID, []uint{from, to}).
		Order("version_no ASC").Find(&versions).Error
	if err != nil {
		return nil, nil, err
	}
	if len(versions) != 2 {
		return nil, nil, model.ErrArticleVersionNotFound
	}
	return &versions[0], &versions[1], nil
}

func (r *ArticleRepository) ListByOwner(userID uint, page, limit int, status string) ([]model.Article, int64, error) {
	query := database.DB.Model(&model.Article{}).Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var articles []model.Article
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("User").Preload("Tags").Offset((page - 1) * limit).Limit(limit).
		Order("created_at DESC, id DESC").Find(&articles).Error
	return articles, total, err
}

func requireArticleSnapshot(tx *gorm.DB, articleID, version uint) error {
	var snapshot model.ArticleVersion
	err := tx.Where("article_id = ? AND version_no = ?", articleID, version).First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrArticleSnapshotMissing
	}
	return err
}

func (r *ArticleRepository) PublishArticle(userID, articleID, expectedVersion uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		article, err := findOwnedArticleForUpdate(tx, userID, articleID)
		if err != nil {
			return err
		}
		if article.Status == model.ArticleStatusArchived {
			return model.ErrArticleArchivedPublish
		}
		if err := checkExpectedVersion(article, expectedVersion); err != nil {
			return err
		}
		if err := requireArticleSnapshot(tx, article.ID, article.Version); err != nil {
			return err
		}
		return tx.Model(article).Updates(map[string]interface{}{
			"status":            model.ArticleStatusPublished,
			"published_version": article.Version,
		}).Error
	})
}

// 重复归档保持幂等，并保留最后一次发布的版本指针。
func (r *ArticleRepository) ArchiveArticle(userID, articleID uint) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		article, err := findOwnedArticle(tx, userID, articleID)
		if err != nil {
			return err
		}
		if article.Status == model.ArticleStatusArchived {
			return nil
		}
		return tx.Model(article).Update("status", model.ArticleStatusArchived).Error
	})
}

// IncrementViewCount 增加浏览量
func (r *ArticleRepository) IncrementViewCount(id uint) error {
	return database.DB.Model(&model.Article{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// UpdateLikeCount 更新点赞数
func (r *ArticleRepository) UpdateLikeCount(id uint, count int) error {
	return database.DB.Model(&model.Article{}).Where("id = ?", id).
		Update("like_count", count).Error
}

// UpdateCommentCount 更新评论数
func (r *ArticleRepository) UpdateCommentCount(id uint, count int) error {
	return database.DB.Model(&model.Article{}).Where("id = ?", id).
		Update("comment_count", count).Error
}

// Count 统计文章总数
func (r *ArticleRepository) Count() int64 {
	var count int64
	database.DB.Model(&model.Article{}).Count(&count)
	return count
}
