package utils

import (
	"strings"
	"testing"
)

// buildSvgPage wraps per-glyph <text> elements the way the ebook API returns
// them: one positioned element per character, all on the same baseline.
func buildSvgPage(style string, glyphs ...string) string {
	var sb strings.Builder
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600">`)
	for i, g := range glyphs {
		sb.WriteString(`<text x="`)
		sb.WriteString(strings.TrimSpace(itoa(40 + i*16)))
		sb.WriteString(`" y="100" style="`)
		sb.WriteString(style)
		sb.WriteString(`">`)
		sb.WriteString(g)
		sb.WriteString(`</text>`)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// Each glyph arrives as its own SVG element, so the converter used to emit
// <b>第</b><b>1</b><b>章</b> — one tag per character. That breaks line
// breaking and search in readers, and bloated a single book to ~18k tags.
// A run of identically formatted glyphs must collapse into one tag pair.
func TestOneByOneHtmlMergesBoldRun(t *testing.T) {
	page := buildSvgPage(
		"font-size:19px;font-weight: bold;fill:rgb(0,0,0);",
		"第", "1", "章", "专", "注",
	)
	content := &SvgContent{Contents: []string{page}, ChapterID: "ch1"}

	result, _, err := OneByOneHtml(eBookTypeEpub, 1, content, nil)
	if err != nil {
		t.Fatalf("OneByOneHtml error = %v", err)
	}

	if got := strings.Count(result, "<b>"); got != 1 {
		t.Errorf("<b> count = %d, want 1 (one run, not one per glyph)\noutput: %s", got, result)
	}
	if got := strings.Count(result, "</b>"); got != 1 {
		t.Errorf("</b> count = %d, want 1", got)
	}
	if !strings.Contains(result, "第1章专注") {
		t.Errorf("glyph run was not contiguous in output: %s", result)
	}
}

func TestOneByOneHtmlMergesItalicRun(t *testing.T) {
	page := buildSvgPage(
		"font-size:16px;font-style: oblique;fill:rgb(0,0,0);",
		"J", "o", "h", "n",
	)
	content := &SvgContent{Contents: []string{page}, ChapterID: "ch1"}

	result, _, err := OneByOneHtml(eBookTypeEpub, 1, content, nil)
	if err != nil {
		t.Fatalf("OneByOneHtml error = %v", err)
	}

	if got := strings.Count(result, "<i>"); got != 1 {
		t.Errorf("<i> count = %d, want 1\noutput: %s", got, result)
	}
	// "John" splitting into <i>J</i><i>o</i>... let readers break mid-word.
	if !strings.Contains(result, "John") {
		t.Errorf("word was split across tags: %s", result)
	}
}

// A genuine style change still has to re-tag, and the tags have to stay
// balanced — merging must not swallow a transition.
func TestOneByOneHtmlRetagsOnFormatChange(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600">`)
	bold := `font-size:16px;font-weight: bold;fill:rgb(0,0,0);`
	plain := `font-size:16px;fill:rgb(0,0,0);`
	for i, g := range []string{"粗", "体"} {
		sb.WriteString(`<text x="` + itoa(40+i*16) + `" y="100" style="` + bold + `">` + g + `</text>`)
	}
	for i, g := range []string{"正", "常"} {
		sb.WriteString(`<text x="` + itoa(80+i*16) + `" y="100" style="` + plain + `">` + g + `</text>`)
	}
	sb.WriteString(`</svg>`)

	content := &SvgContent{Contents: []string{sb.String()}, ChapterID: "ch1"}
	result, _, err := OneByOneHtml(eBookTypeEpub, 1, content, nil)
	if err != nil {
		t.Fatalf("OneByOneHtml error = %v", err)
	}

	if open, close := strings.Count(result, "<b>"), strings.Count(result, "</b>"); open != close {
		t.Errorf("unbalanced bold tags: %d open, %d close\noutput: %s", open, close, result)
	}
	if got := strings.Count(result, "<b>"); got != 1 {
		t.Errorf("<b> count = %d, want 1 for a single bold run\noutput: %s", got, result)
	}
	if !strings.Contains(result, "粗体") {
		t.Errorf("bold run not contiguous: %s", result)
	}
	if strings.Contains(result, "<b>正") || strings.Contains(result, "常</b>") {
		t.Errorf("plain glyphs leaked into the bold run: %s", result)
	}
}

// The source SVG hardcodes an absolute pixel size on every span, which
// overrides the reader's own font-size control — the book cannot be resized
// at all. Sizes must become relative, with the publisher's hierarchy intact.
func TestRelativeFontSizesKeepsHierarchy(t *testing.T) {
	// 16px dominates (body text), 19px is a heading, 13px a caption
	doc := `<span style="font-size:16px;">正文一</span>` +
		`<span style="font-size:16px;">正文二</span>` +
		`<span style="font-size:16px;">正文三</span>` +
		`<span style="font-size:19px;">标题</span>` +
		`<span style="font-size:13px;">注释</span>`

	got := relativeFontSizes(doc)

	if strings.Contains(got, "px") {
		t.Errorf("absolute px survived, reader cannot resize: %s", got)
	}
	if !strings.Contains(got, "font-size:1em") {
		t.Errorf("dominant body size did not become the 1em baseline: %s", got)
	}
	// 19/16 = 1.1875, 13/16 = 0.8125 — hierarchy preserved, not flattened
	if !strings.Contains(got, "font-size:1.188em") {
		t.Errorf("heading lost its relative size: %s", got)
	}
	if !strings.Contains(got, "font-size:0.812em") {
		t.Errorf("caption lost its relative size: %s", got)
	}
}

func TestRelativeFontSizesLeavesDocumentsWithoutSizes(t *testing.T) {
	doc := `<p>没有字号声明</p>`
	if got := relativeFontSizes(doc); got != doc {
		t.Errorf("relativeFontSizes(%q) = %q, want unchanged", doc, got)
	}
}

func TestRelativeFontSizesHandlesSpacingAndDecimals(t *testing.T) {
	// body size must hold the majority, otherwise the tie-break decides the base
	doc := `<span style="font-size: 16px;">a</span>` +
		`<span style="font-size:16px;">b</span>` +
		`<span style="font-size:24.0px;">c</span>`
	got := relativeFontSizes(doc)
	if strings.Contains(got, "px") {
		t.Errorf("px survived: %s", got)
	}
	if !strings.Contains(got, "font-size:1em") {
		t.Errorf("base not normalised: %s", got)
	}
	if !strings.Contains(got, "font-size:1.5em") {
		t.Errorf("24px against a 16px base should be 1.5em: %s", got)
	}
}

// When two sizes appear equally often the larger one becomes the baseline, so
// a handful of captions never demote the body text to a fractional size.
func TestRelativeFontSizesTieBreaksToLargerBase(t *testing.T) {
	doc := `<span style="font-size:16px;">a</span><span style="font-size:12px;">b</span>`
	got := relativeFontSizes(doc)
	if !strings.Contains(got, "font-size:1em") {
		t.Errorf("no baseline chosen: %s", got)
	}
	if !strings.Contains(got, "font-size:0.75em") {
		t.Errorf("12px against a 16px base should be 0.75em: %s", got)
	}
}
