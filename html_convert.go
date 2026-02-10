package main

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	xhtml "golang.org/x/net/html"
)

// htmlToPlain converts HTML to plain text with lightweight markdown-ish markup.
// It preserves paragraphs, line breaks, headings, lists, bold/italic, code/pre, and links.
// It intentionally skips <img> src embedding by default.
func htmlToPlain(htmlSrc string) (string, error) {
	doc, err := xhtml.Parse(strings.NewReader(htmlSrc))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	var listStack []string // "ul" or "ol"
	var olCounters []int

	var walk func(node *xhtml.Node)
	walk = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			// collapse whitespace but keep newlines produced by blocks
			text := html.UnescapeString(n.Data)
			// trim leading/trailing spaces unless inside <pre> or code block
			if parentIsPre(n) {
				buf.WriteString(text)
			} else {
				// collapse internal whitespace to single space
				space := false
				for _, r := range text {
					if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
						space = true
					} else {
						if space {
							buf.WriteByte(' ')
							space = false
						}
						buf.WriteRune(r)
					}
				}
			}

		case xhtml.ElementNode:
			tag := strings.ToLower(n.Data)
			switch tag {
			case "br":
				buf.WriteString("\n")
			case "p":
				// ensure blank line before paragraph unless at very start
				ensureTwoNewlines(&buf)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				ensureTwoNewlines(&buf)
				return
			case "div":
				// treat like paragraph-ish block
				ensureTwoNewlines(&buf)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				ensureTwoNewlines(&buf)
				return
			case "h1", "h2", "h3", "h4", "h5", "h6":
				ensureTwoNewlines(&buf)
				// heading -> prefix with #s
				level := 1
				if len(tag) > 1 {
					fmt.Sscanf(tag[1:], "%d", &level)
				}
				buf.WriteString(strings.Repeat("#", level) + " ")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				ensureTwoNewlines(&buf)
				return
			case "strong", "b":
				buf.WriteString(" **")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				buf.WriteString("**")
				return
			case "em", "i":
				buf.WriteString(" *")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				buf.WriteString("*")
				return
			case "a":
				// collect inner text and href
				var inner bytes.Buffer
				buf.WriteString(" ")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					collectText(&inner, c)
				}
				href := ""
				for _, attr := range n.Attr {
					if strings.ToLower(attr.Key) == "href" {
						href = strings.TrimSpace(attr.Val)
						break
					}
				}
				plainText := strings.TrimSpace(inner.String())
				if href == "" || href == plainText {
					buf.WriteString(plainText)
				} else {
					// format: text (url)
					buf.WriteString(plainText)
					buf.WriteString(" (")
					buf.WriteString(href)
					buf.WriteString(")")
				}
				return
			case "ul":
				// unordered list
				listStack = append(listStack, "ul")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				listStack = listStack[:len(listStack)-1]
				ensureTwoNewlines(&buf)
				return
			case "ol":
				listStack = append(listStack, "ol")
				olCounters = append(olCounters, 1)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				if len(olCounters) > 0 {
					olCounters = olCounters[:len(olCounters)-1]
				}
				listStack = listStack[:len(listStack)-1]
				ensureTwoNewlines(&buf)
				return
			case "li":
				// prefix depending on list type
				prefix := "- "
				if len(listStack) > 0 && listStack[len(listStack)-1] == "ol" {
					// use top ol counter
					if len(olCounters) > 0 {
						i := len(olCounters) - 1
						prefix = fmt.Sprintf("%d. ", olCounters[i])
						olCounters[i]++
					}
				}
				// indent based on depth
				indent := strings.Repeat("  ", len(listStack)-1)
				buf.WriteString(indent + prefix)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				buf.WriteString("\n")
				return
			case "pre":
				ensureTwoNewlines(&buf)
				buf.WriteString("```\n")
				// dump raw text nodes inside pre
				raw := gatherInnerText(n)
				buf.WriteString(raw)
				if !strings.HasSuffix(raw, "\n") {
					buf.WriteString("\n")
				}
				buf.WriteString("```\n")
				ensureTwoNewlines(&buf)
				return
			case "code":
				// inline code: wrap in backticks unless parent is pre
				if parentIsPre(n) {
					// handled by pre
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						walk(c)
					}
					return
				}
				buf.WriteString(" `")
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				buf.WriteString("`")
				return
			case "img":
				// skip images by default; optionally include alt text
				alt := ""
				src := ""
				for _, a := range n.Attr {
					k := strings.ToLower(a.Key)
					if k == "alt" {
						alt = a.Val
					} else if k == "src" {
						src = a.Val
					}
				}
				if alt != "" {
					buf.WriteString(" ![" + alt + "](" + src + ")")
				}
				return
			default:
				// generic: descend
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				return
			}
		default:
			// descend
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}

	walk(doc)
	out := strings.TrimSpace(buf.String())
	// Normalize multi-blank lines to two newlines
	out = normalizeBlankLines(out)
	return out, nil
}

// helper: write two newlines if buffer doesn't already end with one
func ensureTwoNewlines(buf *bytes.Buffer) {
	s := buf.String()
	if strings.HasSuffix(s, "\n\n") {
		return
	}
	if strings.HasSuffix(s, "\n") {
		buf.WriteString("\n")
		return
	}
	buf.WriteString("\n\n")
}

// helper: collect text nodes into a buffer (used for anchors)
func collectText(buf *bytes.Buffer, n *xhtml.Node) {
	if n == nil {
		return
	}
	if n.Type == xhtml.TextNode {
		buf.WriteString(html.UnescapeString(n.Data))
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(buf, c)
	}
}

// gatherInnerText returns the concatenated text inside a node (used for <pre>)
func gatherInnerText(n *xhtml.Node) string {
	var b bytes.Buffer
	var g func(*xhtml.Node)
	g = func(x *xhtml.Node) {
		if x.Type == xhtml.TextNode {
			b.WriteString(x.Data)
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			g(c)
		}
	}
	g(n)
	return b.String()
}

// parentIsPre detects if any ancestor is <pre>
func parentIsPre(n *xhtml.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == xhtml.ElementNode && strings.ToLower(p.Data) == "pre" {
			return true
		}
	}
	return false
}

// normalizeBlankLines collapse >2 blank-lines into exactly 2
func normalizeBlankLines(s string) string {
	// replace 3+ newlines with exactly 2
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
