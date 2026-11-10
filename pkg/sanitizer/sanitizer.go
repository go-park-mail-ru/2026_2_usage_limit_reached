package sanitizer

import (
	"errors"
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

type SanitizerConfig struct {
	AllowedTags        []string          `mapstructure:"allowed_tags"`
	TagAliases         map[string]string `mapstructure:"tag_aliases"`
	RemovedWithContent []string          `mapstructure:"removed_with_content"`
}

func DefaultConfig() SanitizerConfig {
	return SanitizerConfig{
		AllowedTags: []string{"p", "h3", "strong", "em", "u", "s", "ul", "ol", "li", "a", "br"},
		TagAliases: map[string]string{
			"b": "strong", "i": "em", "del": "s", "strike": "s", "div": "p",
		},
		RemovedWithContent: []string{
			"script", "style", "iframe", "object", "embed", "template",
			"noscript", "svg", "math", "textarea", "select",
		},
	}
}

func (c SanitizerConfig) Validate() error {
	var errs []error

	if len(c.AllowedTags) == 0 {
		errs = append(errs, errors.New("sanitizer: allowed_tags must not be empty"))
	}

	allowed := make(map[string]bool, len(c.AllowedTags))
	for _, name := range c.AllowedTags {
		if atom.Lookup([]byte(name)) == 0 {
			errs = append(errs, fmt.Errorf("sanitizer: unknown tag %q in allowed_tags", name))
		}
		allowed[name] = true
	}

	for from, to := range c.TagAliases {
		if atom.Lookup([]byte(from)) == 0 {
			errs = append(errs, fmt.Errorf("sanitizer: unknown tag %q in tag_aliases", from))
		}
		if !allowed[to] {
			errs = append(errs, fmt.Errorf("sanitizer: tag_aliases target %q (for %q) is not in allowed_tags", to, from))
		}
	}

	for _, name := range c.RemovedWithContent {
		if atom.Lookup([]byte(name)) == 0 {
			errs = append(errs, fmt.Errorf("sanitizer: unknown tag %q in removed_with_content", name))
		}
	}

	return errors.Join(errs...)
}

type Sanitizer struct {
	allowedTags        map[atom.Atom]bool
	tagAliases         map[atom.Atom]atom.Atom
	removedWithContent map[atom.Atom]bool
}

func NewSanitizer(cfg SanitizerConfig) (*Sanitizer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	s := &Sanitizer{
		allowedTags:        make(map[atom.Atom]bool, len(cfg.AllowedTags)),
		tagAliases:         make(map[atom.Atom]atom.Atom, len(cfg.TagAliases)),
		removedWithContent: make(map[atom.Atom]bool, len(cfg.RemovedWithContent)),
	}
	for _, name := range cfg.AllowedTags {
		s.allowedTags[atom.Lookup([]byte(name))] = true
	}
	for from, to := range cfg.TagAliases {
		s.tagAliases[atom.Lookup([]byte(from))] = atom.Lookup([]byte(to))
	}
	for _, name := range cfg.RemovedWithContent {
		s.removedWithContent[atom.Lookup([]byte(name))] = true
	}

	return s, nil
}

var bodyContext = &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}

func (s *Sanitizer) Sanitize(input string) (string, error) {
	nodes, err := html.ParseFragment(strings.NewReader(input), bodyContext)
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var output strings.Builder
	for _, node := range nodes {
		for _, tree := range s.cleanNode(node) {
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
func (s *Sanitizer) cleanNode(node *html.Node) []*html.Node {
	switch {
	case node.Type == html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: node.Data}}
	case node.Type != html.ElementNode || s.removedWithContent[node.DataAtom]:
		return nil
	}

	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, s.cleanNode(child)...)
	}

	tag := node.DataAtom
	if alias, ok := s.tagAliases[tag]; ok {
		tag = alias
	}
	if !s.allowedTags[tag] {
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
