package repository

import (
	"errors"

	"blog-system/internal/database"
	"blog-system/internal/model"

	"gorm.io/gorm"
)

// ApplyArticleEdit 只接受应用的显式批准；事务内重新读取当前内容，不信任提案中的其他字段。
func (r *ArticleRepository) ApplyArticleEdit(userID, articleID, baseVersion uint, content string) (*model.Article, error) {
	var applied model.Article
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		current, err := findOwnedArticleForUpdate(tx, userID, articleID)
		if err != nil {
			return err
		}
		if err := checkExpectedVersion(current, baseVersion); err != nil {
			return err
		}
		if current.Status == model.ArticleStatusArchived {
			return model.ErrArticleArchivedEdit
		}
		if err := requireArticleSnapshot(tx, articleID, baseVersion); err != nil {
			if errors.Is(err, model.ErrArticleSnapshotMissing) {
				return model.ErrArticleVersionNotFound
			}
			return err
		}
		if err := model.ValidateArticleContent(content); err != nil {
			return err
		}
		if current.Content == content {
			return model.ErrArticleContentUnchanged
		}
		applied = *current
		applied.Content = content
		return saveArticleContent(tx, current, &applied, nil, false, userID, false)
	})
	if err != nil {
		return nil, err
	}
	return &applied, nil
}
