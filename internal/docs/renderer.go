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

func RenderMarkdown(raw []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := md.Convert(raw, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
