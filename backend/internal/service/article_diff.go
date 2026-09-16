package service

import "errors"

var ErrInvalidDiffVersions = errors.New("必须满足 0 < from_version < to_version")

type TextFieldChange struct {
	Changed bool   `json:"changed"`
	Before  string `json:"before"`
	After   string `json:"after"`
}

type ArticleFieldChanges struct {
	Title      TextFieldChange `json:"title"`
	Summary    TextFieldChange `json:"summary"`
	CoverImage TextFieldChange `json:"cover_image"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// 索引是各自完整归一化序列中的位置，从 0 开始；缺失的一侧返回 null。
type ContentBlockChange struct {
	Operation   string        `json:"operation"`
	BeforeIndex *int          `json:"before_index"`
	AfterIndex  *int          `json:"after_index"`
	Before      *ContentBlock `json:"before"`
	After       *ContentBlock `json:"after"`
}

type ContentChanges struct {
	Changed bool                 `json:"changed"`
	Changes []ContentBlockChange `json:"changes"`
}

type ArticleVersionDiff struct {
	ArticleID    uint                `json:"article_id"`
	FromVersion  uint                `json:"from_version"`
	ToVersion    uint                `json:"to_version"`
	FieldChanges ArticleFieldChanges `json:"field_changes"`
	Content      ContentChanges      `json:"content"`
}

func (s *ArticleService) GetVersionDiff(userID, articleID, from, to uint) (*ArticleVersionDiff, error) {
	if from == 0 || from >= to {
		return nil, ErrInvalidDiffVersions
	}
	before, after, err := s.articleRepo.FindOwnedVersions(userID, articleID, from, to)
	if err != nil {
		return nil, err
	}
	a, err := normalizeArticleHTML(before.Content)
	if err != nil {
		return nil, err
	}
	b, err := normalizeArticleHTML(after.Content)
	if err != nil {
		return nil, err
	}
	field := func(a, b string) TextFieldChange { return TextFieldChange{a != b, a, b} }
	changes := compareContentBlocks(a, b)
	return &ArticleVersionDiff{
		ArticleID: articleID, FromVersion: from, ToVersion: to,
		FieldChanges: ArticleFieldChanges{
			Title: field(before.Title, after.Title), Summary: field(before.Summary, after.Summary),
			CoverImage: field(before.CoverImage, after.CoverImage),
		},
		Content: ContentChanges{Changed: len(changes) > 0, Changes: changes},
	}, nil
}

type blockMatch struct{ before, after int }

// Hirschberg LCS 的时间复杂度为 O(n×m)，使用线性行存储，避免分配完整二维矩阵。
// 分数相同时选择目标序列中最早的分割点，确保重复块的对齐结果确定。
func matchingBlocks(a, b []ContentBlock, aOffset, bOffset int) []blockMatch {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	if len(a) == 1 {
		for j := range b {
			if a[0] == b[j] {
				return []blockMatch{{aOffset, bOffset + j}}
			}
		}
		return nil
	}
	mid := len(a) / 2
	split := lcsSplit(a[:mid], a[mid:], b)
	return append(matchingBlocks(a[:mid], b[:split], aOffset, bOffset),
		matchingBlocks(a[mid:], b[split:], aOffset+mid, bOffset+split)...)
}

func lcsSplit(left, right, b []ContentBlock) int {
	forward, backward := lcsRow(left, b, false), lcsRow(right, b, true)
	split, best := 0, -1
	for j := 0; j <= len(b); j++ {
		if score := forward[j] + backward[len(b)-j]; score > best {
			split, best = j, score
		}
	}
	return split
}

func lcsRow(a, b []ContentBlock, reverse bool) []int {
	row := make([]int, len(b)+1)
	for i := range a {
		diagonal := 0
		for j := range b {
			x, y := i, j
			if reverse {
				x, y = len(a)-1-i, len(b)-1-j
			}
			previous := row[j+1]
			if a[x] == b[y] {
				row[j+1] = diagonal + 1
			} else if row[j] > row[j+1] {
				row[j+1] = row[j]
			}
			diagonal = previous
		}
	}
	return row
}

func compareContentBlocks(a, b []ContentBlock) []ContentBlockChange {
	changes := make([]ContentBlockChange, 0)
	matches := append(matchingBlocks(a, b, 0, 0), blockMatch{len(a), len(b)})
	i, j := 0, 0
	for _, match := range matches {
		// 未匹配块仅在类型相同时按位置配对，不推测语义相似度。
		for i < match.before || j < match.after {
			change := ContentBlockChange{}
			if i < match.before && j < match.after && a[i].Type == b[j].Type {
				x, y := i, j
				change = ContentBlockChange{"modify", &x, &y, &a[i], &b[j]}
				i++
				j++
			} else if i < match.before {
				x := i
				change = ContentBlockChange{Operation: "delete", BeforeIndex: &x, Before: &a[i]}
				i++
			} else {
				y := j
				change = ContentBlockChange{Operation: "insert", AfterIndex: &y, After: &b[j]}
				j++
			}
			changes = append(changes, change)
		}
		i, j = match.before+1, match.after+1
	}
	return changes
}
