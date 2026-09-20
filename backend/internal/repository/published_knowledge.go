package repository

import (
	"blog-system/internal/database"
	"blog-system/internal/model"
)

// 复用公开快照选择条件，不读取工作正文，也不增加浏览量。
// 单条 SQL 同时绑定生命周期、公开指针和对应快照。
func (r *ArticleRepository) PublishedKnowledgeArticles(articleID uint) ([]model.Article, error) {
	query := publishedArticleQuery(database.DB).Select(publishedArticleColumns)
	if articleID != 0 {
		query = query.Where("articles.id = ?", articleID)
	}
	var articles []model.Article
	err := query.Order("articles.id ASC").Find(&articles).Error
	return articles, err
}
