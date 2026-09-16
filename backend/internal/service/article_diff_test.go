package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeArticleHTML(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        []ContentBlock
	}{
		{"blocks", `<h2>标题 &amp; 标记</h2><p>Redis<strong>很快</strong><br>真的</p><ul><li><p>第一项</p></li><li>第二项</li></ul><blockquote><p>引用</p></blockquote><pre><code>if x &lt; 2:\n  pass</code></pre>`, []ContentBlock{
			{"heading", "标题 & 标记"}, {"paragraph", "Redis很快 真的"}, {"list_item", "第一项"}, {"list_item", "第二项"}, {"blockquote", "引用"}, {"code_block", `if x < 2:\n  pass`},
		}},
		{"nested", `<ul><li><p>父项</p><ul><li><p>子项</p></li></ul><p>后文</p></li></ul><blockquote><p>甲</p><p>乙</p></blockquote>`, []ContentBlock{
			{"list_item", "父项"}, {"list_item", "子项"}, {"list_item", "后文"}, {"blockquote", "甲"}, {"blockquote", "乙"},
		}},
		{"whitespace", "<p>  a\n b&nbsp;c </p><pre><code> x\r\n  y\n</code></pre>", []ContentBlock{{"paragraph", "a b c"}, {"code_block", " x\n  y\n"}}},
		{"wrappers", `<div>one <em>two</em></div><div>three</div>`, []ContentBlock{{"paragraph", "one two"}, {"paragraph", "three"}}},
		{"empty", `<p></p><p><br></p>`, []ContentBlock{{"paragraph", ""}, {"paragraph", ""}}},
		{"unclosed", `<p>first<p>second`, []ContentBlock{{"paragraph", "first"}, {"paragraph", "second"}}},
		{"non-content", `<script>secret()</script><style>p{}</style><!-- ignore --><template>hidden</template>`, []ContentBlock{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeArticleHTML(tc.input)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, err=%v; want %#v", got, err, tc.want)
			}
		})
	}
}

func TestContentBlockDiff(t *testing.T) {
	for _, tc := range []struct {
		name, before, after         string
		operation, oldText, newText string
		oldIndex, newIndex          int
	}{
		{"identical", "<p>Redis</p>", "<p>Redis</p>", "", "", "", -1, -1},
		{"formatting", "<p>Redis</p>", `<p class="a"><strong>Redis</strong></p>`, "", "", "", -1, -1},
		{"paragraph", "<p>Redis很快</p>", "<p>Redis性能很高</p>", "modify", "Redis很快", "Redis性能很高", 0, 0},
		{"insert", "<p>A</p><p>C</p>", "<p>A</p><p>B</p><p>C</p>", "insert", "", "B", -1, 1},
		{"delete", "<p>A</p><p>B</p><p>C</p>", "<p>A</p><p>C</p>", "delete", "B", "", 1, -1},
		{"insert empty document", "", "<h1>A</h1>", "insert", "", "A", -1, 0},
		{"delete empty document", "<p>A</p>", "", "delete", "A", "", 0, -1},
		{"code whitespace", "<pre>x\n y</pre>", "<pre>x\n  y</pre>", "modify", "x\n y", "x\n  y", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := normalizeArticleHTML(tc.before)
			if err != nil {
				t.Fatal(err)
			}
			b, err := normalizeArticleHTML(tc.after)
			if err != nil {
				t.Fatal(err)
			}
			got := compareContentBlocks(a, b)
			if tc.operation == "" {
				if len(got) != 0 || got == nil {
					t.Fatalf("expected empty changes: %#v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Operation != tc.operation {
				t.Fatalf("changes=%+v", got)
			}
			c := got[0]
			if tc.oldIndex < 0 {
				if c.Before != nil || c.BeforeIndex != nil {
					t.Fatal("insert must have null before")
				}
			} else if c.Before == nil || c.Before.Text != tc.oldText || c.BeforeIndex == nil || *c.BeforeIndex != tc.oldIndex {
				t.Fatalf("before=%+v", c)
			}
			if tc.newIndex < 0 {
				if c.After != nil || c.AfterIndex != nil {
					t.Fatal("delete must have null after")
				}
			} else if c.After == nil || c.After.Text != tc.newText || c.AfterIndex == nil || *c.AfterIndex != tc.newIndex {
				t.Fatalf("after=%+v", c)
			}
		})
	}
}

func TestBlockTypeAndRepeatedBlockDiff(t *testing.T) {
	a := []ContentBlock{{"paragraph", "A"}, {"paragraph", "A"}, {"paragraph", "B"}}
	b := []ContentBlock{{"paragraph", "A"}, {"paragraph", "B"}, {"paragraph", "A"}}
	first, _ := json.Marshal(compareContentBlocks(a, b))
	for i := 0; i < 20; i++ {
		next, _ := json.Marshal(compareContentBlocks(a, b))
		if string(first) != string(next) {
			t.Fatal("non-deterministic alignment")
		}
	}
	changes := compareContentBlocks([]ContentBlock{{"heading", "A"}}, []ContentBlock{{"paragraph", "A"}})
	if len(changes) != 2 || changes[0].Operation != "delete" || changes[1].Operation != "insert" {
		t.Fatalf("type changes must not disappear: %+v", changes)
	}
}

// 使用小规模穷举得到的基准结果校验 LCS 长度，覆盖重复块。
func TestMatchingBlocksAlignment(t *testing.T) {
	sequences := [][]ContentBlock{{}}
	for n := 1; n <= 4; n++ {
		for mask := 0; mask < 1<<n; mask++ {
			s := make([]ContentBlock, n)
			for i := range s {
				s[i] = ContentBlock{"paragraph", string(rune('A' + ((mask >> i) & 1)))}
			}
			sequences = append(sequences, s)
		}
	}
	var oracle func([]ContentBlock, []ContentBlock) int
	oracle = func(a, b []ContentBlock) int {
		if len(a) == 0 || len(b) == 0 {
			return 0
		}
		if a[0] == b[0] {
			return 1 + oracle(a[1:], b[1:])
		}
		x, y := oracle(a[1:], b), oracle(a, b[1:])
		if x > y {
			return x
		}
		return y
	}
	for _, a := range sequences {
		for _, b := range sequences {
			matches := matchingBlocks(a, b, 0, 0)
			if len(matches) != oracle(a, b) {
				t.Fatalf("wrong LCS: %v %v", a, b)
			}
			last := blockMatch{-1, -1}
			for _, m := range matches {
				if m.before <= last.before || m.after <= last.after || a[m.before] != b[m.after] {
					t.Fatal("invalid alignment")
				}
				last = m
			}
		}
	}
}
