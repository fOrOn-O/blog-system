package service

import (
	"errors"
	"time"

	"blog-system/internal/model"
	"blog-system/internal/repository"
)

// CommentService 评论服务
type CommentService struct {
	commentRepo *repository.CommentRepository
	articleRepo *repository.ArticleRepository
}

// NewCommentService 创建评论服务
func NewCommentService() *CommentService {
	return &CommentService{
		commentRepo: repository.NewCommentRepository(),
		articleRepo: repository.NewArticleRepository(),
	}
}

// CreateCommentRequest 创建评论请求
type CreateCommentRequest struct {
	Content  string `json:"content" binding:"required"`
	ParentID *uint  `json:"parent_id"`
}

// CommentResponse 评论响应
type CommentResponse struct {
	ID        uint              `json:"id"`
	Content   string            `json:"content"`
	User      *UserResponse     `json:"user,omitempty"`
	ArticleID uint              `json:"article_id"`
	ParentID  *uint             `json:"parent_id,omitempty"`
	Replies   []CommentResponse `json:"replies,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Create 创建评论
func (s *CommentService) Create(userID, articleID uint, req CreateCommentRequest) (*CommentResponse, error) {
	// 检查文章是否存在
	_, err := s.articleRepo.FindByID(articleID)
	if err != nil {
		return nil, errors.New("文章不存在")
	}

	// 如果是回复，检查父评论是否存在且属于同一文章
	if req.ParentID != nil {
		parent, err := s.commentRepo.FindByID(*req.ParentID)
		if err != nil {
			return nil, errors.New("父评论不存在")
		}
		if parent.ArticleID != articleID {
			return nil, errors.New("父评论不属于该文章")
		}
	}

	comment := &model.Comment{
		Content:   req.Content,
		UserID:    userID,
		ArticleID: articleID,
		ParentID:  req.ParentID,
	}

	if err := s.commentRepo.Create(comment); err != nil {
		return nil, errors.New("创建评论失败")
	}

	// 更新文章评论数
	count := s.commentRepo.CountByArticleID(articleID)
	s.articleRepo.UpdateCommentCount(articleID, int(count))

	// 重新加载关联数据
	comment, _ = s.commentRepo.FindByID(comment.ID)
	return toCommentResponse(comment), nil
}

// GetByArticleID 获取文章的评论列表
func (s *CommentService) GetByArticleID(articleID uint, page, limit int) ([]CommentResponse, int64, error) {
	comments, total, err := s.commentRepo.GetByArticleID(articleID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]CommentResponse, 0, len(comments))
	for _, comment := range comments {
		responses = append(responses, *toCommentResponse(&comment))
	}

	return responses, total, nil
}

// Delete 删除评论
func (s *CommentService) Delete(userID, commentID uint) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return errors.New("评论不存在")
	}

	// 检查权限
	if comment.UserID != userID {
		return errors.New("无权删除此评论")
	}

	if err := s.commentRepo.Delete(commentID); err != nil {
		return errors.New("删除评论失败")
	}

	// 更新文章评论数
	count := s.commentRepo.CountByArticleID(comment.ArticleID)
	s.articleRepo.UpdateCommentCount(comment.ArticleID, int(count))

	return nil
}

// toCommentResponse 转换为评论响应
func toCommentResponse(comment *model.Comment) *CommentResponse {
	resp := &CommentResponse{
		ID:        comment.ID,
		Content:   comment.Content,
		ArticleID: comment.ArticleID,
		ParentID:  comment.ParentID,
		CreatedAt: comment.CreatedAt,
	}

	if comment.User.ID > 0 {
		resp.User = toUserResponse(&comment.User)
	}

	// 递归转换回复
	for _, reply := range comment.Replies {
		resp.Replies = append(resp.Replies, *toCommentResponse(&reply))
	}

	return resp
}
