package service

import "errors"

var ErrInvalidArticleVersion = errors.New("文章 ID 和版本号必须为正整数")

// ArticleVersionResponse 仅返回指定历史版本的内容，不混入当前工作状态或标签。
type ArticleVersionResponse struct {
	ArticleID  uint   `json:"article_id"`
	VersionNo  uint   `json:"version_no"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
}

func (s *ArticleService) GetOwnedVersion(userID, articleID, versionNo uint) (*ArticleVersionResponse, error) {
	if articleID == 0 || versionNo == 0 {
		return nil, ErrInvalidArticleVersion
	}
	version, err := s.articleRepo.FindOwnedVersion(userID, articleID, versionNo)
	if err != nil {
		return nil, err
	}
	return &ArticleVersionResponse{
		ArticleID: version.ArticleID, VersionNo: version.VersionNo,
		Title: version.Title, Content: version.Content, Summary: version.Summary, CoverImage: version.CoverImage,
	}, nil
}
