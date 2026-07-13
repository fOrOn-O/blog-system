package service

import (
	"errors"
	"time"

	"blog-system/internal/model"
	"blog-system/internal/repository"
)

// FavoriteService 收藏服务
type FavoriteService struct {
	favoriteRepo *repository.FavoriteRepository
	articleRepo  *repository.ArticleRepository
}

// NewFavoriteService 创建收藏服务
func NewFavoriteService() *FavoriteService {
	return &FavoriteService{
		favoriteRepo: repository.NewFavoriteRepository(),
		articleRepo:  repository.NewArticleRepository(),
	}
}

// FavoriteResponse 收藏响应
type FavoriteResponse struct {
	IsFavorited bool `json:"is_favorited"`
}

// FavoriteArticleResponse 收藏的文章响应
type FavoriteArticleResponse struct {
	ID        uint           `json:"id"`
	Article   *ArticleResponse `json:"article"`
	CreatedAt time.Time      `json:"created_at"`
}

// Favorite 收藏文章
func (s *FavoriteService) Favorite(userID, articleID uint) error {
	// 检查文章是否存在
	_, err := s.articleRepo.FindByID(articleID)
	if err != nil {
		return errors.New("文章不存在")
	}

	// 检查是否已收藏
	if s.favoriteRepo.Exists(userID, articleID) {
		return errors.New("已经收藏过了")
	}

	return s.favoriteRepo.Create(&model.Favorite{
		UserID:    userID,
		ArticleID: articleID,
	})
}

// Unfavorite 取消收藏
func (s *FavoriteService) Unfavorite(userID, articleID uint) error {
	// 检查是否已收藏
	if !s.favoriteRepo.Exists(userID, articleID) {
		return errors.New("未收藏")
	}

	return s.favoriteRepo.Delete(userID, articleID)
}

// IsFavorited 检查是否已收藏
func (s *FavoriteService) IsFavorited(userID, articleID uint) *FavoriteResponse {
	return &FavoriteResponse{
		IsFavorited: s.favoriteRepo.Exists(userID, articleID),
	}
}

// GetUserFavorites 获取用户收藏列表
func (s *FavoriteService) GetUserFavorites(userID uint, page, limit int) ([]FavoriteArticleResponse, int64, error) {
	favorites, total, err := s.favoriteRepo.GetByUserID(userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var responses []FavoriteArticleResponse
	for _, fav := range favorites {
		resp := FavoriteArticleResponse{
			ID:        fav.ID,
			CreatedAt: fav.CreatedAt,
		}
		if fav.Article.ID > 0 {
			resp.Article = toArticleResponse(&fav.Article)
		}
		responses = append(responses, resp)
	}

	return responses, total, nil
}
