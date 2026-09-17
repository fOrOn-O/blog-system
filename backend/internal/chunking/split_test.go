package chunking

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitBoundaryPreferences(t *testing.T) {
	for _, tc := range []struct {
		name, text    string
		code          bool
		max, overflow int
		want          []string
	}{
		{"English sentences", "One. Two. Three.", false, 9, 0, []string{"One. ", "Two. ", "Three."}},
		{"closing quote", "Hello!\" Next? Done.", false, 8, 0, []string{"Hello!\" ", "Next? ", "Done."}},
		{"sentence overflow", "1234567890。abcdefghij。尾", false, 10, 2, []string{"1234567890。", "abcdefghij。尾"}},
		{"line before sentence", "a.b\nc!d\ne?f\n", true, 7, 0, []string{"a.b\n", "c!d\n", "e?f\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := splitBlock(tc.text, tc.code, Options{TargetSize: tc.max, MaxSize: tc.max, MaxOverflow: tc.overflow})
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

// 覆盖小尺寸和混合边界，验证最终兜底不会丢字、重复文本、破坏 Unicode 或超出配置额度。
func TestSplitPreservesAllRunes(t *testing.T) {
	for _, source := range []string{
		strings.Repeat("中😀abc", 19),
		strings.Repeat("甲。乙！丙？ ", 17),
		strings.Repeat("Hi. \"Why?!\" End. ", 13),
		strings.Repeat("  x\n\n    longer line\n", 11),
	} {
		for max := 1; max <= 24; max++ {
			for overflow := 0; overflow <= 3; overflow++ {
				for _, code := range []bool{false, true} {
					options := Options{TargetSize: max, MaxSize: max, MaxOverflow: overflow}
					pieces := splitBlock(source, code, options)
					if strings.Join(pieces, "") != source {
						t.Fatal("split lost or duplicated text")
					}
					for _, piece := range pieces {
						if piece == "" || !utf8.ValidString(piece) || textSize(piece) > max+overflow {
							t.Fatalf("invalid piece %q for %+v", piece, options)
						}
					}
				}
			}
		}
	}
}
