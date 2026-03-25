package docs

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.NewTable(),
		extension.Strikethrough,
		extension.TaskList,
	),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
	),
)

// RenderMarkdown converts Markdown to safe HTML. Raw HTML in the source is
// stripped by goldmark's default behavior (no html.WithUnsafe). The result is
// cast to template.HTML to bypass Go's auto-escaping. DO NOT enable
// html.WithUnsafe without adding an HTML sanitizer (e.g., bluemonday)
// between goldmark output and the template.HTML cast. README content comes
// from third-party repos and must be treated as untrusted.
func RenderMarkdown(raw []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := md.Convert(raw, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
