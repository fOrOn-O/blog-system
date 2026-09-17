package chunking

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func chunkForTest(t *testing.T, content string, options Options) []Chunk {
	t.Helper()
	chunks, err := ChunkHTML(content, options)
	if err != nil {
		t.Fatal(err)
	}
	return chunks
}

func TestHeadingHierarchy(t *testing.T) {
	content := `<p>前言</p><h1>Redis</h1><h2>Persistence</h2><h3>RDB</h3><p>Snapshots</p><h3>AOF</h3><p>Commands</p><h2>Cluster</h2><p>Nodes</p><h1>Next</h1><p>New root</p>`
	chunks := chunkForTest(t, content, DefaultOptions())
	want := [][]Heading{
		{}, {{1, "Redis"}, {2, "Persistence"}, {3, "RDB"}},
		{{1, "Redis"}, {2, "Persistence"}, {3, "AOF"}}, {{1, "Redis"}, {2, "Cluster"}}, {{1, "Next"}},
	}
	if len(chunks) != len(want) {
		t.Fatalf("chunks=%+v", chunks)
	}
	for i, chunk := range chunks {
		if chunk.Index != i || !reflect.DeepEqual(chunk.HeadingPath, want[i]) {
			t.Fatalf("chunk[%d]=%+v", i, chunk)
		}
	}
}

func TestAllHeadingLevelsAndSkippedLevels(t *testing.T) {
	var content strings.Builder
	for level := 1; level <= 6; level++ {
		fmt.Fprintf(&content, "<h%d>L%d</h%d><p>P%d</p>", level, level, level, level)
	}
	chunks := chunkForTest(t, content.String(), DefaultOptions())
	if len(chunks) != 6 {
		t.Fatalf("chunks=%+v", chunks)
	}
	for i, chunk := range chunks {
		if len(chunk.HeadingPath) != i+1 {
			t.Fatalf("path=%+v", chunk.HeadingPath)
		}
		for j, heading := range chunk.HeadingPath {
			if heading.Level != j+1 || heading.Text != fmt.Sprintf("L%d", j+1) {
				t.Fatalf("heading=%+v", heading)
			}
		}
	}
	skipped := chunkForTest(t, `<h2>Start</h2><h5>Deep</h5><p>A</p><h4>Back</h4><p>B</p>`, DefaultOptions())
	if !reflect.DeepEqual(skipped[0].HeadingPath, []Heading{{2, "Start"}, {5, "Deep"}}) || !reflect.DeepEqual(skipped[1].HeadingPath, []Heading{{2, "Start"}, {4, "Back"}}) {
		t.Fatalf("skipped=%+v", skipped)
	}
}

func TestSemanticBlocksAndIgnoredNodes(t *testing.T) {
	content := `<head><title>HEAD_SECRET</title></head><h2>Hi <em>there</em></h2><p>Redis<strong>很快</strong><span> &amp; safe</span></p><ul><li><p>parent</p><ul><li>child</li></ul></li></ul><blockquote><p>quote</p></blockquote><pre><code>if x &lt; 2:
  pass
</code></pre><script>SCRIPT_SECRET</script><style>STYLE_SECRET</style><template><h1>HIDDEN_HEADING</h1>TEMPLATE_SECRET</template><!-- COMMENT_SECRET -->`
	chunks := chunkForTest(t, content, DefaultOptions())
	want := []ChunkBlock{{"paragraph", "Redis很快 & safe"}, {"list_item", "parent"}, {"list_item", "child"}, {"blockquote", "quote"}, {"code_block", "if x < 2:\n  pass\n"}}
	if len(chunks) != 1 || !reflect.DeepEqual(chunks[0].Blocks, want) || !reflect.DeepEqual(chunks[0].HeadingPath, []Heading{{2, "Hi there"}}) {
		t.Fatalf("chunks=%+v", chunks)
	}
}

func TestCompatibleBlocksMergeAndHeadingChangesSplit(t *testing.T) {
	options := Options{TargetSize: 6, MaxSize: 12}
	chunks := chunkForTest(t, `<h1>A</h1><p>甲乙</p><p>丙丁</p><p>戊己</p><h2>B</h2><p>x</p><h2>C</h2><p>y</p>`, options)
	if len(chunks) != 3 || len(chunks[0].Blocks) != 3 || textSize(RenderText(Chunk{Blocks: chunks[0].Blocks})) != 10 {
		t.Fatalf("chunks=%+v", chunks)
	}
	if chunks[1].HeadingPath[1].Text != "B" || chunks[2].HeadingPath[1].Text != "C" {
		t.Fatal("merged across headings")
	}
}

func TestTargetAndMaxPolicies(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		options       Options
		blockCounts   []int
	}{
		{"target is soft", `<p>12345678</p><p>123456</p><p>xy</p>`, Options{TargetSize: 10, MaxSize: 18}, []int{2, 1}},
		{"max starts chunk", `<p>12345678</p><p>abcdefgh</p>`, Options{TargetSize: 10, MaxSize: 12}, []int{1, 1}},
		{"small tail merges", `<p>123456</p><p>xy</p>`, Options{TargetSize: 6, MaxSize: 10}, []int{2}},
		{"lower target", strings.Repeat(`<p>1234</p>`, 4), Options{TargetSize: 4, MaxSize: 12}, []int{1, 1, 2}},
		{"higher target", strings.Repeat(`<p>1234</p>`, 4), Options{TargetSize: 8, MaxSize: 12}, []int{2, 2}},
		{"unicode counts runes", `<p>中文😀</p><p>内容🌏</p>`, Options{TargetSize: 6, MaxSize: 8}, []int{2}},
		{"overflow isolated", `<p>12345678901</p><p>x</p>`, Options{TargetSize: 6, MaxSize: 10, MaxOverflow: 2}, []int{1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkForTest(t, tc.content, tc.options)
			counts := make([]int, len(chunks))
			for i, c := range chunks {
				counts[i] = len(c.Blocks)
			}
			if !reflect.DeepEqual(counts, tc.blockCounts) {
				t.Fatalf("counts=%v want=%v chunks=%+v", counts, tc.blockCounts, chunks)
			}
		})
	}
}

func TestFallbackPiecesAndNoOverlap(t *testing.T) {
	options := Options{TargetSize: 6, MaxSize: 10, MaxOverflow: 2}
	for _, tc := range []struct {
		name, tag, text string
		want            []string
	}{
		{"sentences", "p", "甲乙丙丁。戊己庚辛。壬癸子丑。", []string{"甲乙丙丁。戊己庚辛。", "壬癸子丑。"}},
		{"runes", "p", strings.Repeat("中😀", 13), []string{strings.Repeat("中😀", 5), strings.Repeat("中😀", 5), strings.Repeat("中😀", 3)}},
		{"normal code", "pre", " x\n  y", []string{" x\n  y"}},
		{"slight code overflow", "pre", "123456789012", []string{"123456789012"}},
		{"code lines", "pre", strings.Repeat("abc\n", 8), []string{"abc\nabc\n", "abc\nabc\n", "abc\nabc\n", "abc\nabc\n"}},
		{"long code line", "pre", strings.Repeat("界", 25), []string{strings.Repeat("界", 10), strings.Repeat("界", 10), strings.Repeat("界", 5)}},
		{"line overflow preserves line", "pre", "1234567890\nabcdefghi\nxyz", []string{"1234567890\n", "abcdefghi\n", "xyz"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkForTest(t, "<h2>Context</h2><"+tc.tag+">"+tc.text+"</"+tc.tag+">", options)
			var texts []string
			for i, c := range chunks {
				if c.Index != i || !reflect.DeepEqual(c.HeadingPath, []Heading{{2, "Context"}}) {
					t.Fatalf("chunk=%+v", c)
				}
				for _, block := range c.Blocks {
					if !utf8.ValidString(block.Text) {
						t.Fatal("split damaged Unicode")
					}
					texts = append(texts, block.Text)
				}
			}
			if !reflect.DeepEqual(texts, tc.want) || strings.Join(texts, "") != tc.text {
				t.Fatalf("texts=%q want=%q", texts, tc.want)
			}
		})
	}
}

func TestEmptyDeterminismAndIndependentSlices(t *testing.T) {
	for _, input := range []string{"", `<h1>Title only</h1><p></p><p><br></p>`, `<script>ignored</script>`} {
		chunks := chunkForTest(t, input, DefaultOptions())
		if chunks == nil || len(chunks) != 0 {
			t.Fatalf("empty input=%+v", chunks)
		}
	}
	input := `<h1>Root</h1><h2>Child</h2><p>12345678</p><p>abcdefgh</p><h2>Next</h2><p>tail</p>`
	options := Options{TargetSize: 8, MaxSize: 10}
	first := chunkForTest(t, input, options)
	encoded, _ := json.Marshal(first)
	for i := 0; i < 20; i++ {
		next, _ := json.Marshal(chunkForTest(t, input, options))
		if string(encoded) != string(next) {
			t.Fatal("non-deterministic output")
		}
	}
	first[0].HeadingPath[0].Text = "changed"
	first[0].Blocks[0].Text = "changed"
	if first[1].HeadingPath[0].Text != "Root" || first[1].Blocks[0].Text != "abcdefgh" {
		t.Fatal("shared mutable slice")
	}
}

func TestRenderTextAndInvalidOptions(t *testing.T) {
	chunk := Chunk{HeadingPath: []Heading{{1, "Redis"}, {2, "Persistence"}, {3, "RDB"}}, Blocks: []ChunkBlock{{"paragraph", "Snapshots"}, {"paragraph", "Recovery"}}}
	if got := RenderText(chunk); got != "Redis > Persistence > RDB\n\nSnapshots\n\nRecovery" {
		t.Fatalf("rendered=%q", got)
	}
	chunk.Blocks[0].Text = "Updated"
	if !strings.Contains(RenderText(chunk), "Updated") {
		t.Fatal("render used stale stored content")
	}
	for _, options := range []Options{{}, {TargetSize: 0, MaxSize: 10}, {TargetSize: 11, MaxSize: 10}, {TargetSize: 1, MaxSize: 10, MaxOverflow: -1}, {TargetSize: 1, MaxSize: int(^uint(0) >> 1), MaxOverflow: 1}} {
		if _, err := ChunkHTML("<p>text</p>", options); err == nil {
			t.Fatalf("accepted invalid options %+v", options)
		}
	}
}
