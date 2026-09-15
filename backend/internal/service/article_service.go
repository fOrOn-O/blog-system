package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/internal/repository"

	"gorm.io/gorm"
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

// CreateDraftRequest 只包含内容字段，不具备发布或归档能力。
type CreateDraftRequest struct {
	Title      string `json:"title" binding:"required,max=200"`
	Content    string `json:"content" binding:"required"`
	Summary    string `json:"summary"`
	CoverImage string `json:"cover_image"`
	TagIDs     []uint `json:"tag_ids"`
}

// UpdateDraftRequest 只修改工作内容。
type UpdateDraftRequest struct {
	ExpectedVersion uint    `json:"expected_version" binding:"required,min=1"`
	Title           *string `json:"title" binding:"omitempty,min=1,max=200"`
	Content         *string `json:"content" binding:"omitempty,min=1"`
	Summary         *string `json:"summary"`
	CoverImage      *string `json:"cover_image"`
	TagIDs          *[]uint `json:"tag_ids"`
}

// 旧人类接口共享内容字段，但业务方法固定为立即发布。
type CreateArticleRequest CreateDraftRequest
type UpdateArticleRequest UpdateDraftRequest

// PublishArticleRequest 明确调用方批准公开的工作版本。
type PublishArticleRequest struct {
	ExpectedVersion uint `json:"expected_version" binding:"required,min=1"`
}

// TagResponse 文章标签响应
type TagResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ArticleResponse 文章响应
type ArticleResponse struct {
	ID               uint          `json:"id"`
	Version          uint          `json:"version"`
	PublishedVersion uint          `json:"published_version"`
	Title            string        `json:"title"`
	Content          string        `json:"content"`
	Summary          string        `json:"summary"`
	CoverImage       string        `json:"cover_image"`
	User             *UserResponse `json:"user,omitempty"`
	ViewCount        int           `json:"view_count"`
	LikeCount        int           `json:"like_count"`
	CommentCount     int           `json:"comment_count"`
	Status           string        `json:"status"`
	Tags             []TagResponse `json:"tags,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// Create 创建文章
func (s *ArticleService) Create(userID uint, req CreateArticleRequest) (*ArticleResponse, error) {
	return s.createArticle(userID, CreateDraftRequest(req), model.ArticleStatusPublished)
}

func (s *ArticleService) CreateDraft(userID uint, req CreateDraftRequest) (*ArticleResponse, error) {
	return s.createArticle(userID, req, model.ArticleStatusDraft)
}

func (s *ArticleService) createArticle(userID uint, req CreateDraftRequest, status string) (*ArticleResponse, error) {
	tags, err := s.resolveTags(req.TagIDs)
	if err != nil {
		return nil, err
	}

	// 自动生成摘要
	summary := req.Summary
	if summary == "" && len(req.Content) > 200 {
		summary = req.Content[:200] + "..."
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
	invalidateArticleCache(article.ID)

	// 重新加载关联数据
	return s.GetOwnedArticle(userID, article.ID)
}

// GetByID 获取文章详情
func (s *ArticleService) GetByID(id uint) (*ArticleResponse, error) {
	// 数据库决定当前是否公开，缓存不能绕过状态与发布版本检查。
	article, err := s.articleRepo.FindPublishedByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrArticleNotFound
		}
		return nil, err
	}
	cacheKey := fmt.Sprintf("article:public:v2:%d", id)
	if cached, err := database.CacheGet(cacheKey); err == nil {
		var resp ArticleResponse
		if err := json.Unmarshal([]byte(cached), &resp); err == nil && samePublicRevision(resp, article) {
			return &resp, nil
		}
	}

	// 公共读取在返回前完成计数；Owner 读取不执行此操作。
	if err := s.articleRepo.IncrementViewCount(id); err != nil {
		return nil, err
	}

	resp := toArticleResponse(article)

	// 写入缓存
	if data, err := json.Marshal(resp); err == nil {
		database.CacheSet(cacheKey, string(data), 10*time.Minute)
	}

	return resp, nil
}

// List 获取文章列表
func (s *ArticleService) List(page, limit int, _ string) ([]ArticleResponse, int64, error) {
	articles, total, err := s.articleRepo.List(page, limit)
	if err != nil {
		return nil, 0, err
	}
	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("articles:list:public:v2:%d:%d", page, limit)
	if cached, err := database.CacheGet(cacheKey); err == nil {
		var result struct {
			Articles []ArticleResponse `json:"articles"`
			Total    int64             `json:"total"`
		}
		if err := json.Unmarshal([]byte(cached), &result); err == nil && result.Total == total && len(result.Articles) == len(articles) {
			valid := true
			for i := range articles {
				if !samePublicRevision(result.Articles[i], &articles[i]) {
					valid = false
					break
				}
			}
			if valid {
				return result.Articles, result.Total, nil
			}
		}
	}

	responses := make([]ArticleResponse, 0, len(articles))
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

func samePublicRevision(cached ArticleResponse, article *model.Article) bool {
	return cached.ID == article.ID && cached.Status == model.ArticleStatusPublished &&
		cached.Version == article.PublishedVersion && cached.PublishedVersion == article.PublishedVersion &&
		cached.UpdatedAt.Equal(article.UpdatedAt)
}

// Search 搜索文章
func (s *ArticleService) Search(keyword string, page, limit int) ([]ArticleResponse, int64, error) {
	articles, total, err := s.articleRepo.Search(keyword, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]ArticleResponse, 0, len(articles))
	for _, article := range articles {
		responses = append(responses, *toArticleResponse(&article))
	}

	return responses, total, nil
}

// Update 更新文章
func (s *ArticleService) Update(userID, articleID uint, req UpdateArticleRequest) (*ArticleResponse, error) {
	return s.updateArticle(userID, articleID, UpdateDraftRequest(req), true)
}

func (s *ArticleService) UpdateDraft(userID, articleID uint, req UpdateDraftRequest) (*ArticleResponse, error) {
	return s.updateArticle(userID, articleID, req, false)
}

func (s *ArticleService) updateArticle(userID, articleID uint, req UpdateDraftRequest, publish bool) (*ArticleResponse, error) {
	article, err := s.articleRepo.FindOwnedByID(userID, articleID)
	if err != nil {
		return nil, err
	}

	if article.Status == model.ArticleStatusArchived {
		return nil, model.ErrArticleArchivedEdit
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
	if publish {
		err = s.articleRepo.UpdateAndPublish(article, tags, replaceTags, userID, req.ExpectedVersion)
	} else {
		err = s.articleRepo.UpdateDraft(article, tags, replaceTags, userID, req.ExpectedVersion)
	}
	if err != nil {
		return nil, err
	}

	// 清除缓存
	invalidateArticleCache(articleID)

	return s.GetOwnedArticle(userID, articleID)
}

func (s *ArticleService) GetOwnedArticle(userID, articleID uint) (*ArticleResponse, error) {
	article, err := s.articleRepo.FindOwnedByID(userID, articleID)
	if err != nil {
		return nil, err
	}
	return toArticleResponse(article), nil
}

func (s *ArticleService) ListMyArticles(userID uint, page, limit int, status string) ([]ArticleResponse, int64, error) {
	if status != "" && status != model.ArticleStatusDraft && status != model.ArticleStatusPublished && status != model.ArticleStatusArchived {
		return nil, 0, errors.New("无效的文章状态")
	}
	articles, total, err := s.articleRepo.ListByOwner(userID, page, limit, status)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]ArticleResponse, 0, len(articles))
	for i := range articles {
		responses = append(responses, *toArticleResponse(&articles[i]))
	}
	return responses, total, nil
}

func (s *ArticleService) PublishArticle(userID, articleID uint, req PublishArticleRequest) (*ArticleResponse, error) {
	if err := s.articleRepo.PublishArticle(userID, articleID, req.ExpectedVersion); err != nil {
		return nil, err
	}
	invalidateArticleCache(articleID)
	return s.GetOwnedArticle(userID, articleID)
}

func (s *ArticleService) ArchiveArticle(userID, articleID uint) (*ArticleResponse, error) {
	if err := s.articleRepo.ArchiveArticle(userID, articleID); err != nil {
		return nil, err
	}
	invalidateArticleCache(articleID)
	return s.GetOwnedArticle(userID, articleID)
}

func invalidateArticleCache(articleID uint) {
	database.CacheDelete(fmt.Sprintf("article:%d", articleID))
	database.CacheDelete(fmt.Sprintf("article:public:v2:%d", articleID))
	database.CacheDeletePrefix("articles:list:")
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
	invalidateArticleCache(articleID)

	return nil
}

// toArticleResponse 转换为文章响应
func toArticleResponse(article *model.Article) *ArticleResponse {
	resp := &ArticleResponse{
		ID:               article.ID,
		Version:          article.Version,
		PublishedVersion: article.PublishedVersion,
		Title:            article.Title,
		Content:          article.Content,
		Summary:          article.Summary,
		CoverImage:       article.CoverImage,
		ViewCount:        article.ViewCount,
		LikeCount:        article.LikeCount,
		CommentCount:     article.CommentCount,
		Status:           article.Status,
		CreatedAt:        article.CreatedAt,
		UpdatedAt:        article.UpdatedAt,
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
