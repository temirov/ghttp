package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var converter = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// ToHTML converts Markdown text to HTML using a common configuration.
func ToHTML(source []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := converter.Convert(source, &buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
