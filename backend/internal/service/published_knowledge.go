package service

import "blog-system/internal/chunking"

// 已发布知识只包含公开快照；不携带账号信息或未发布的工作版本。
type PublishedKnowledgeRecord struct {
	ArticleID        uint                   `json:"article_id"`
	Title            string                 `json:"title"`
	PublishedVersion uint                   `json:"published_version"`
	Chunks           []ArticleChunkResponse `json:"chunks"`
}

func (s *ArticleService) PublishedKnowledge(articleID uint) ([]PublishedKnowledgeRecord, error) {
	articles, err := s.articleRepo.PublishedKnowledgeArticles(articleID)
	if err != nil {
		return nil, err
	}
	result := make([]PublishedKnowledgeRecord, 0, len(articles))
	for _, article := range articles {
		chunks, err := chunking.ChunkHTML(article.Content, chunking.DefaultOptions())
		if err != nil {
			return nil, err
		}
		record := PublishedKnowledgeRecord{ArticleID: article.ID, Title: article.Title,
			PublishedVersion: article.PublishedVersion, Chunks: make([]ArticleChunkResponse, 0, len(chunks))}
		for _, chunk := range chunks {
			record.Chunks = append(record.Chunks, ArticleChunkResponse{Chunk: chunk, Text: chunking.RenderText(chunk)})
		}
		result = append(result, record)
	}
	return result, nil
}
