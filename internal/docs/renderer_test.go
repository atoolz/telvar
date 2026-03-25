package docs

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBasic(t *testing.T) {
	raw := []byte("# Hello\n\nThis is **bold** and *italic*.\n")
	html, err := RenderMarkdown(raw)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	s := string(html)
	if !strings.Contains(s, "<h1>Hello</h1>") {
		t.Errorf("expected h1, got: %s", s)
	}
	if !strings.Contains(s, "<strong>bold</strong>") {
		t.Errorf("expected strong, got: %s", s)
	}
	if !strings.Contains(s, "<em>italic</em>") {
		t.Errorf("expected em, got: %s", s)
	}
}

func TestRenderMarkdownGFMTable(t *testing.T) {
	raw := []byte("| A | B |\n|---|---|\n| 1 | 2 |\n")
	html, err := RenderMarkdown(raw)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	s := string(html)
	if !strings.Contains(s, "<table>") {
		t.Errorf("expected table, got: %s", s)
	}
}

func TestRenderMarkdownCodeBlock(t *testing.T) {
	raw := []byte("```go\nfunc main() {}\n```\n")
	html, err := RenderMarkdown(raw)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	s := string(html)
	if !strings.Contains(s, "<code") {
		t.Errorf("expected code block, got: %s", s)
	}
}

func TestRenderMarkdownStrikethrough(t *testing.T) {
	raw := []byte("~~deleted~~\n")
	html, err := RenderMarkdown(raw)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	s := string(html)
	if !strings.Contains(s, "<del>deleted</del>") {
		t.Errorf("expected del, got: %s", s)
	}
}

func TestRenderMarkdownNoRawHTML(t *testing.T) {
	raw := []byte("<script>alert('xss')</script>\n")
	html, err := RenderMarkdown(raw)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	s := string(html)
	if strings.Contains(s, "<script>") {
		t.Errorf("raw HTML should be escaped or stripped, got: %s", s)
	}
}

func TestRenderMarkdownEmpty(t *testing.T) {
	html, err := RenderMarkdown([]byte(""))
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if string(html) != "" {
		t.Errorf("expected empty output, got: %q", html)
	}
}
