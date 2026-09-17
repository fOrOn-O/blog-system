package htmlcontent

import (
	"reflect"
	"testing"
)

func TestSharedExtractionKeepsHeadingLevel(t *testing.T) {
	input := `<h1>Root</h1><h6><strong>Deep</strong></h6><p>Text &amp; more</p><ul><li><p>item</p></li></ul>`
	want := []Block{
		{Type: "heading", Text: "Root", HeadingLevel: 1},
		{Type: "heading", Text: "Deep", HeadingLevel: 6},
		{Type: "paragraph", Text: "Text & more"},
		{Type: "list_item", Text: "item"},
	}
	for _, parse := range []func(string) ([]Block, error){Parse, ParseFragment} {
		got, err := parse(input)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	}
}

func TestDocumentExcludesHeadWithoutChangingFragmentMode(t *testing.T) {
	input := `<head><title>Metadata</title></head><h2>Body</h2><p>Visible</p>`
	document, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document, []Block{{Type: "heading", Text: "Body", HeadingLevel: 2}, {Type: "paragraph", Text: "Visible"}}) {
		t.Fatalf("document=%+v", document)
	}
	// Diff 原先使用片段解析；抽取共享能力时不悄然改变这一兼容行为。
	fragment, err := ParseFragment(input)
	if err != nil || len(fragment) != 3 || fragment[0].Text != "Metadata" {
		t.Fatalf("fragment mode changed: %+v %v", fragment, err)
	}
}
