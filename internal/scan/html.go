package scan

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/webxsid/pdg/internal/state"
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
			name := attr(node, "name")
			content := strings.TrimSpace(attr(node, "content"))
			switch name {
			case "pdg:title":
				doc.Title = content
			case "pdg:description":
				doc.Description = content
			case "pdg:published-at":
				publishedAt = content
			case "pdg:updated-at":
				if content != "" {
					t, parseErr := time.Parse(time.RFC3339, content)
					if parseErr != nil {
						return
					}
					doc.UpdatedAt = &t
				}
			case "pdg:tag":
				if content != "" {
					doc.Tags = append(doc.Tags, content)
				}
			case "pdg:targets":
				doc.TargetsExplicit = true
				if content != "" {
					targets, parseErr := parseTargets(content)
					if parseErr == nil {
						doc.Targets = targets
					}
				}
			}
			if attr(node, "property") == "og:description" && doc.Description == "" {
				doc.Description = content
			}
			if attr(node, "property") == "og:title" && doc.Title == "" {
				doc.Title = content
			}
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

func parseTargets(content string) ([]state.PublicationTarget, error) {
	parts := strings.Split(content, ",")
	seen := map[state.PublicationTarget]bool{}
	result := make([]state.PublicationTarget, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, errors.New("empty target")
		}
		if value == "none" {
			if len(parts) != 1 {
				return nil, errors.New("none must be used alone")
			}
			return nil, nil
		}
		var target state.PublicationTarget
		switch value {
		case string(state.TargetStandardSite):
			target = state.TargetStandardSite
		case string(state.TargetBluesky):
			target = state.TargetBluesky
		default:
			return nil, fmt.Errorf("unknown target %q", value)
		}
		if !seen[target] {
			seen[target] = true
			result = append(result, target)
		}
	}
	return result, nil
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
