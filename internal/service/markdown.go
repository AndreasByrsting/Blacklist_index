package service

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var md = goldmark.New(goldmark.WithExtensions(extension.GFM))

// RenderMarkdown 将 Markdown 渲染为安全 HTML（goldmark 默认转义原始 HTML）。
func RenderMarkdown(src string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return buf.String()
}

var (
	reCodeBlock    = regexp.MustCompile("(?s)```.*?```")
	reImage        = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	reLink         = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading      = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+`)
	reBlockquote   = regexp.MustCompile(`(?m)^\s{0,3}>\s?`)
	reOrderedList  = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`)
	reUnordered    = regexp.MustCompile(`(?m)^\s*[-*+]\s+`)
	reHR           = regexp.MustCompile(`(?m)^\s*(?:-{3,}|\*{3,}|_{3,})\s*$`)
	reBoldItalic   = regexp.MustCompile(`[*_~]{1,3}`)
	reSpaces       = regexp.MustCompile(`[ \t]+`)
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
)

// MarkdownToPlain 将 Markdown 转换为纯文本（用于列表预览）。
func MarkdownToPlain(src string) string {
	s := src
	s = reCodeBlock.ReplaceAllString(s, "")
	s = reImage.ReplaceAllString(s, "$1")
	s = reLink.ReplaceAllString(s, "$1")
	s = reBoldItalic.ReplaceAllString(s, "")
	s = reHeading.ReplaceAllString(s, "")
	s = reBlockquote.ReplaceAllString(s, "")
	s = reOrderedList.ReplaceAllString(s, "")
	s = reUnordered.ReplaceAllString(s, "")
	s = reHR.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "\\", "")
	s = reSpaces.ReplaceAllString(s, " ")
	s = reMultiNewline.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
