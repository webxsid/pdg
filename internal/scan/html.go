package scan

import (
	"errors"
	"io"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func ParseHTML(r io.Reader) (Document, error) {
	root, err := html.Parse(r)
	if err != nil {
		return Document{}, err
	}

	var doc Document
	var publishedAt string

	walk(root, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}

		switch node.Data {
		case "title":
			doc.Title = textContent(node)

		case "link":
			if attr(node, "rel") == "canonical" {
				doc.CanonicalURL = attr(node, "href")
			}

		case "meta":
			if attr(node, "property") == "article:published_time" {
				publishedAt = attr(node, "content")
			}

		case "article":
			doc.TextContent = textContent(node)

		}
	})

	if publishedAt != "" {
		t, err := time.Parse(time.RFC3339, publishedAt)
		if err != nil {
			return Document{}, errors.New("failed to parse published time: " + err.Error())
		}
		doc.PublishedAt = t
	}

	if doc.Title == "" {
		return Document{}, errors.New("missing title")
	}

	if doc.CanonicalURL == "" {
		return Document{}, errors.New("missing canonical URL")
	}

	if doc.TextContent == "" {
		return Document{}, errors.New("missing text content")
	}

	return doc, nil
}

func walk(node *html.Node, visit func(*html.Node)) {
	visit(node)

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walk(child, visit)
	}
}

func attr(node *html.Node, name string) string {
	for _, a := range node.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	walk(node, func(node *html.Node) {
		if node.Type != html.TextNode {
			return
		}
		value := strings.TrimSpace(node.Data)
		if value == "" {
			return
		}
		if builder.Len() > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(value)
	})
	return builder.String()
}
