package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/repository"
)

// ArticleService 文章服务
type ArticleService struct {
	articleRepo *repository.ArticleRepository
	tagRepo     *repository.TagRepository
}

// NewArticleService 创建文章服务
func NewArticleService() *ArticleService {
	return &ArticleService{
		articleRepo: repository.NewArticleRepository(),
		tagRepo:     repository.NewTagRepository(),
	}
}

// CreateArticleRequest 创建文章请求
type CreateArticleRequest struct {
	Title      string `json:"title" binding:"required,max=200"`
	Content    string `json:"content" binding:"required"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
	Status     string `json:"status" binding:"omitempty,oneof=draft published archived"`
	TagIDs     []uint `json:"tag_ids"`
}

// UpdateArticleRequest 更新文章请求
type UpdateArticleRequest struct {
	Title      *string `json:"title" binding:"omitempty,min=1,max=200"`
	Content    *string `json:"content" binding:"omitempty,min=1"`
	Summary    *string `json:"summary"`
	CoverImage *string `json:"cover_image"`
	Status     *string `json:"status" binding:"omitempty,oneof=draft published archived"`
	TagIDs     *[]uint `json:"tag_ids"`
}

// TagResponse 文章标签响应
type TagResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ArticleResponse 文章响应
type ArticleResponse struct {
	ID           uint          `json:"id"`
	Title        string        `json:"title"`
	Content      string        `json:"content"`
	Summary      string        `json:"summary"`
	CoverImage   string        `json:"cover_image"`
	User         *UserResponse `json:"user,omitempty"`
	ViewCount    int           `json:"view_count"`
	LikeCount    int           `json:"like_count"`
	CommentCount int           `json:"comment_count"`
	Status       string        `json:"status"`
	Tags         []TagResponse `json:"tags,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// Create 创建文章
func (s *ArticleService) Create(userID uint, req CreateArticleRequest) (*ArticleResponse, error) {
	tags, err := s.resolveTags(req.TagIDs)
	if err != nil {
		return nil, err
	}

	// 自动生成摘要
	summary := req.Summary
	if summary == "" && len(req.Content) > 200 {
		summary = req.Content[:200] + "..."
	}

	// 设置默认状态
	status := req.Status
	if status == "" {
		status = "published"
	}

	article := &model.Article{
		Title:      req.Title,
		Content:    req.Content,
		Summary:    summary,
		CoverImage: req.CoverImage,
		UserID:     userID,
		Status:     status,
	}

	if err := s.articleRepo.Create(article, tags); err != nil {
		return nil, errors.New("创建文章失败")
	}

	// 清除缓存
	database.CacheDeletePrefix("articles:list:")

	// 重新加载关联数据
	article, _ = s.articleRepo.FindByID(article.ID)
	return toArticleResponse(article), nil
}

// GetByID 获取文章详情
func (s *ArticleService) GetByID(id uint) (*ArticleResponse, error) {
	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("article:%d", id)
	if cached, err := database.CacheGet(cacheKey); err == nil {
		var resp ArticleResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil {
			return &resp, nil
		}
	}

	article, err := s.articleRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("文章不存在")
	}

	// 异步增加浏览量
	go s.articleRepo.IncrementViewCount(id)

	resp := toArticleResponse(article)

	// 写入缓存
	if data, err := json.Marshal(resp); err == nil {
		database.CacheSet(cacheKey, string(data), 10*time.Minute)
	}

	return resp, nil
}

// List 获取文章列表
func (s *ArticleService) List(page, limit int, status string) ([]ArticleResponse, int64, error) {
	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("articles:list:%d:%d:%s", page, limit, status)
	if cached, err := database.CacheGet(cacheKey); err == nil {
		var result struct {
			Articles []ArticleResponse `json:"articles"`
			Total    int64             `json:"total"`
		}
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return result.Articles, result.Total, nil
		}
	}

	articles, total, err := s.articleRepo.List(page, limit, status)
	if err != nil {
		return nil, 0, err
	}

	var responses []ArticleResponse
	for _, article := range articles {
		responses = append(responses, *toArticleResponse(&article))
	}

	// 写入缓存
	if data, err := json.Marshal(map[string]interface{}{
		"articles": responses,
		"total":    total,
	}); err == nil {
		database.CacheSet(cacheKey, string(data), 5*time.Minute)
	}

	return responses, total, nil
}

// Search 搜索文章
func (s *ArticleService) Search(keyword string, page, limit int) ([]ArticleResponse, int64, error) {
	articles, total, err := s.articleRepo.Search(keyword, page, limit)
	if err != nil {
		return nil, 0, err
	}

	var responses []ArticleResponse
	for _, article := range articles {
		responses = append(responses, *toArticleResponse(&article))
	}

	return responses, total, nil
}

// Update 更新文章
func (s *ArticleService) Update(userID, articleID uint, req UpdateArticleRequest) (*ArticleResponse, error) {
	article, err := s.articleRepo.FindByID(articleID)
	if err != nil {
		return nil, errors.New("文章不存在")
	}

	// 检查权限
	if article.UserID != userID {
		return nil, errors.New("无权修改此文章")
	}

	var tags []model.Tag
	replaceTags := req.TagIDs != nil
	if replaceTags {
		tags, err = s.resolveTags(*req.TagIDs)
		if err != nil {
			return nil, err
		}
	}

	if req.Title != nil {
		article.Title = *req.Title
	}
	if req.Content != nil {
		article.Content = *req.Content
	}
	if req.Summary != nil {
		article.Summary = *req.Summary
	}
	if req.CoverImage != nil {
		article.CoverImage = *req.CoverImage
	}
	if req.Status != nil {
		article.Status = *req.Status
	}

	if err := s.articleRepo.Update(article, tags, replaceTags); err != nil {
		return nil, errors.New("更新文章失败")
	}

	// 清除缓存
	database.CacheDelete(fmt.Sprintf("article:%d", articleID))
	database.CacheDeletePrefix("articles:list:")

	article, err = s.articleRepo.FindByID(articleID)
	if err != nil {
		return nil, errors.New("重新加载文章失败")
	}

	return toArticleResponse(article), nil
}

// resolveTags 校验标签ID，并按照请求顺序返回标签
func (s *ArticleService) resolveTags(tagIDs []uint) ([]model.Tag, error) {
	if len(tagIDs) == 0 {
		return []model.Tag{}, nil
	}

	uniqueIDs := make([]uint, 0, len(tagIDs))
	seen := make(map[uint]struct{}, len(tagIDs))
	for _, id := range tagIDs {
		if id == 0 {
			return nil, errors.New("包含不存在的标签")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	tags, err := s.tagRepo.GetByIDs(uniqueIDs)
	if err != nil {
		return nil, errors.New("获取标签失败")
	}
	if len(tags) != len(uniqueIDs) {
		return nil, errors.New("包含不存在的标签")
	}

	tagsByID := make(map[uint]model.Tag, len(tags))
	for _, tag := range tags {
		tagsByID[tag.ID] = tag
	}

	orderedTags := make([]model.Tag, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		orderedTags = append(orderedTags, tagsByID[id])
	}
	return orderedTags, nil
}

// Delete 删除文章
func (s *ArticleService) Delete(userID uint, role string, articleID uint) error {
	article, err := s.articleRepo.FindByID(articleID)
	if err != nil {
		return errors.New("文章不存在")
	}

	// 检查权限
	if article.UserID != userID && role != "admin" {
		return errors.New("无权删除此文章")
	}

	if err := s.articleRepo.Delete(articleID); err != nil {
		return errors.New("删除文章失败")
	}

	// 清除缓存
	database.CacheDelete(fmt.Sprintf("article:%d", articleID))
	database.CacheDeletePrefix("articles:list:")

	return nil
}

// toArticleResponse 转换为文章响应
func toArticleResponse(article *model.Article) *ArticleResponse {
	resp := &ArticleResponse{
		ID:           article.ID,
		Title:        article.Title,
		Content:      article.Content,
		Summary:      article.Summary,
		CoverImage:   article.CoverImage,
		ViewCount:    article.ViewCount,
		LikeCount:    article.LikeCount,
		CommentCount: article.CommentCount,
		Status:       article.Status,
		CreatedAt:    article.CreatedAt,
		UpdatedAt:    article.UpdatedAt,
	}

	if article.User.ID > 0 {
		resp.User = toUserResponse(&article.User)
	}

	for _, tag := range article.Tags {
		resp.Tags = append(resp.Tags, TagResponse{
			ID:   tag.ID,
			Name: tag.Name,
		})
	}

	return resp
}
