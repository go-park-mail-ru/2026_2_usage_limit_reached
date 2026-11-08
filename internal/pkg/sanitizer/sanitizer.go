package sanitizer

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Правила санитайзинга взяты с фронтенда:

// — Разрешены теги: p, h3, strong, em, u, s, ul, ol, li, a, br.
// — b заменяется на strong, i на em, del и strike на s, div на p.
// — Все переданные атрибуты удаляются, кроме проверенного href у ссылки. То есть style, class, id, onclick и прочее не сохраняются.
// — Ссылки разрешены только с полным https-адресом. Относительные ссылки и адреса с http:, javascript:, data: не разрешены.
// — Ссылкам добавляются target="_blank" и rel="noopener noreferrer nofollow".
// — Если адрес ссылки недопустимый, сам тег ссылки убирается, но её текст остается.
// — script, style, iframe, object, embed, template, noscript, svg, math, textarea, select удаляются вместе с содержимым .
// — У остальных неразрешенных тегов убирается сам тег, а допустимое содержимое сохраняется.
// — HTML-комментарии удаляются. Картинки через img не поддерживаются.

const linkRel = "noopener noreferrer nofollow"

var allowedTags = map[atom.Atom]bool{
	atom.P: true, atom.H3: true, atom.Strong: true, atom.Em: true, atom.U: true, atom.S: true,
	atom.Ul: true, atom.Ol: true, atom.Li: true, atom.A: true, atom.Br: true,
}

var tagAliases = map[atom.Atom]atom.Atom{
	atom.B: atom.Strong, atom.I: atom.Em, atom.Del: atom.S, atom.Strike: atom.S, atom.Div: atom.P,
}

var removedWithContent = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Iframe: true, atom.Object: true, atom.Embed: true, atom.Template: true,
	atom.Noscript: true, atom.Svg: true, atom.Math: true, atom.Textarea: true, atom.Select: true,
}

var bodyContext = &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}

func Sanitize(input string) (string, error) {
	nodes, err := html.ParseFragment(strings.NewReader(input), bodyContext)
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var output strings.Builder
	for _, node := range nodes {
		for _, tree := range cleanNode(node) {
			if err := html.Render(&output, tree); err != nil {
				return "", fmt.Errorf("render html: %w", err)
			}
		}
	}

	return output.String(), nil
}

func HasText(input string) bool {
	tokenizer := html.NewTokenizer(strings.NewReader(input))

	for token := tokenizer.Next(); token != html.ErrorToken; token = tokenizer.Next() {
		if token == html.TextToken && strings.TrimSpace(string(tokenizer.Text())) != "" {
			return true
		}
	}

	return false
}

// Рекурсия безопасна, т.к. html.ParseFragment отклоняет вложенность глубже 512 элементов
func cleanNode(node *html.Node) []*html.Node {
	switch {
	case node.Type == html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: node.Data}}
	case node.Type != html.ElementNode || removedWithContent[node.DataAtom]:
		return nil
	}

	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, cleanNode(child)...)
	}

	tag := node.DataAtom
	if alias, ok := tagAliases[tag]; ok {
		tag = alias
	}
	if !allowedTags[tag] {
		return children
	}

	clean := &html.Node{Type: html.ElementNode, DataAtom: tag, Data: tag.String()}
	if tag == atom.A {
		href, ok := safeURL(attr(node, "href"))
		if !ok {
			return children
		}
		clean.Attr = []html.Attribute{
			{Key: "href", Val: href},
			{Key: "target", Val: "_blank"},
			{Key: "rel", Val: linkRel},
		}
	}

	for _, child := range children {
		clean.AppendChild(child)
	}

	return []*html.Node{clean}
}

func safeURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", false
	}

	return parsed.String(), true
}

func attr(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Namespace == "" && attribute.Key == key {
			return attribute.Val
		}
	}

	return ""
}
