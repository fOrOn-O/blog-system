package service

import (
	"errors"

	"blog-system/internal/model"
	"blog-system/internal/repository"
)

// LikeService 点赞服务
type LikeService struct {
	likeRepo    *repository.LikeRepository
	articleRepo *repository.ArticleRepository
}

// NewLikeService 创建点赞服务
func NewLikeService() *LikeService {
	return &LikeService{
		likeRepo:    repository.NewLikeRepository(),
		articleRepo: repository.NewArticleRepository(),
	}
}

// LikeResponse 点赞响应
type LikeResponse struct {
	Count   int  `json:"count"`
	IsLiked bool `json:"is_liked"`
}

// Like 点赞文章
func (s *LikeService) Like(userID, articleID uint) error {
	// 检查文章是否存在
	_, err := s.articleRepo.FindByID(articleID)
	if err != nil {
		return errors.New("文章不存在")
	}

	// 检查是否已点赞
	if s.likeRepo.Exists(userID, articleID) {
		return errors.New("已经点赞过了")
	}

	like := &model.Like{
		UserID:    userID,
		ArticleID: articleID,
	}

	if err := s.likeRepo.Create(like); err != nil {
		return errors.New("点赞失败")
	}

	// 更新文章点赞数
	count := s.likeRepo.CountByArticleID(articleID)
	s.articleRepo.UpdateLikeCount(articleID, int(count))

	return nil
}

// Unlike 取消点赞
func (s *LikeService) Unlike(userID, articleID uint) error {
	// 检查是否已点赞
	if !s.likeRepo.Exists(userID, articleID) {
		return errors.New("未点赞")
	}

	if err := s.likeRepo.Delete(userID, articleID); err != nil {
		return errors.New("取消点赞失败")
	}

	// 更新文章点赞数
	count := s.likeRepo.CountByArticleID(articleID)
	s.articleRepo.UpdateLikeCount(articleID, int(count))

	return nil
}

// GetLikeInfo 获取点赞信息
func (s *LikeService) GetLikeInfo(userID, articleID uint) *LikeResponse {
	count := s.likeRepo.CountByArticleID(articleID)
	isLiked := false
	if userID > 0 {
		isLiked = s.likeRepo.Exists(userID, articleID)
	}

	return &LikeResponse{
		Count:   int(count),
		IsLiked: isLiked,
	}
}
