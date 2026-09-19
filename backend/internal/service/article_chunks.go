package service

import (
	"errors"

	"blog-system/internal/chunking"
)

var ErrInvalidChunkVersion = errors.New("文章 ID 和版本号必须为正整数")

// Text 仅为 API 派生字段，不写入 Task 09 的 Chunk 模型或数据库。
type ArticleChunkResponse struct {
	chunking.Chunk
	Text string `json:"text"`
}

type ArticleVersionChunks struct {
	UserID    uint                   `json:"user_id"`
	ArticleID uint                   `json:"article_id"`
	VersionNo uint                   `json:"version_no"`
	Chunks    []ArticleChunkResponse `json:"chunks"`
}

func (s *ArticleService) GetVersionChunks(userID, articleID, versionNo uint) (*ArticleVersionChunks, error) {
	if articleID == 0 || versionNo == 0 {
		return nil, ErrInvalidChunkVersion
	}
	version, err := s.articleRepo.FindOwnedVersion(userID, articleID, versionNo)
	if err != nil {
		return nil, err
	}
	chunks, err := chunking.ChunkHTML(version.Content, chunking.DefaultOptions())
	if err != nil {
		return nil, err
	}
	result := &ArticleVersionChunks{
		UserID: userID, ArticleID: version.ArticleID, VersionNo: version.VersionNo,
		Chunks: make([]ArticleChunkResponse, 0, len(chunks)),
	}
	for _, chunk := range chunks {
		result.Chunks = append(result.Chunks, ArticleChunkResponse{Chunk: chunk, Text: chunking.RenderText(chunk)})
	}
	return result, nil
}
