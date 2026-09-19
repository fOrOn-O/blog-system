package service

import "blog-system/internal/model"

// 身份来自 JWT；不接受提案摘要、生命周期、公开版本或客户端指定的新版本号。
type ArticleEditRequest struct {
	BaseVersionNo   uint   `json:"base_version_no" binding:"required,min=1"`
	ProposedContent string `json:"proposed_content" binding:"required,min=1"`
}

// 提案没有持久化的目标版本。复用 Task 08 的差异结构，不虚构 to_version。
type ArticleEditPreview struct {
	ArticleID     uint                `json:"article_id"`
	BaseVersionNo uint                `json:"base_version_no"`
	FieldChanges  ArticleFieldChanges `json:"field_changes"`
	Content       ContentChanges      `json:"content"`
}

type ArticleEditApplied struct {
	ArticleID         uint `json:"article_id"`
	PreviousVersionNo uint `json:"previous_version_no"`
	NewVersionNo      uint `json:"new_version_no"`
	// Status 描述此次保存的草稿结果，不替代 Article.Status 生命周期。
	Status           string `json:"status"`
	PublishedVersion uint   `json:"published_version"`
}

func (s *ArticleService) PreviewArticleEdit(userID, articleID uint, req ArticleEditRequest) (*ArticleEditPreview, error) {
	if articleID == 0 || req.BaseVersionNo == 0 {
		return nil, ErrInvalidArticleVersion
	}
	base, err := s.articleRepo.FindOwnedVersion(userID, articleID, req.BaseVersionNo)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateArticleContent(req.ProposedContent); err != nil {
		return nil, err
	}
	proposed := *base
	proposed.Content = req.ProposedContent
	diff, err := compareArticleVersions(base, &proposed)
	if err != nil {
		return nil, err
	}
	return &ArticleEditPreview{ArticleID: articleID, BaseVersionNo: req.BaseVersionNo,
		FieldChanges: diff.FieldChanges, Content: diff.Content}, nil
}

func (s *ArticleService) ApplyArticleEdit(userID, articleID uint, req ArticleEditRequest) (*ArticleEditApplied, error) {
	if articleID == 0 || req.BaseVersionNo == 0 {
		return nil, ErrInvalidArticleVersion
	}
	article, err := s.articleRepo.ApplyArticleEdit(userID, articleID, req.BaseVersionNo, req.ProposedContent)
	if err != nil {
		return nil, err
	}
	invalidateArticleCache(articleID)
	// 使用此次事务的结果，避免提交后再次查询读到另一请求的更晚版本。
	return &ArticleEditApplied{ArticleID: articleID, PreviousVersionNo: req.BaseVersionNo,
		NewVersionNo: article.Version, Status: model.ArticleStatusDraft,
		PublishedVersion: article.PublishedVersion}, nil
}
