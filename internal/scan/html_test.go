package scan

import (
	"strings"
	"testing"
	"time"
)

func TestParseHTML(t *testing.T) {
	input := `
<!doctype html>
<html>
<head>
	<title>Hello Atmosphere</title>

	<link
		rel="canonical"
		href="https://example.com/blog/hello-atmosphere"
	>

	<meta
		property="article:published_time"
		content="2026-08-23T08:00:00Z"
	>
</head>

<body>
	<article>
		<h1>Hello Atmosphere</h1>
		<p>This is an existing website.</p>
	</article>
</body>
</html>
`

	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML() error = %v", err)
	}

	if doc.Title != "Hello Atmosphere" {
		t.Errorf("Title = %q", doc.Title)
	}

	if doc.CanonicalURL != "https://example.com/blog/hello-atmosphere" {
		t.Errorf("CanonicalURL = %q", doc.CanonicalURL)
	}

	expectedPublishedAt := time.Date(
		2026, 8, 23,
		8, 0, 0, 0,
		time.UTC,
	)

	if !doc.PublishedAt.Equal(expectedPublishedAt) {
		t.Errorf(
			"PublishedAt = %v, want %v",
			doc.PublishedAt,
			expectedPublishedAt,
		)
	}

	expectedText := "Hello Atmosphere This is an existing website."

	if doc.TextContent != expectedText {
		t.Errorf(
			"TextContent = %q, want %q",
			doc.TextContent,
			expectedText,
		)
	}
}

func TestParseHTMLTargetsNone(t *testing.T) {
	input := `<html><head><title>Hidden</title><meta name="pdg:targets" content="none"></head><body><article>Content</article></body></html>`
	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML() error = %v", err)
	}
	if !doc.TargetsExplicit || !doc.TargetsNone {
		t.Fatalf("targets metadata = explicit %v, none %v", doc.TargetsExplicit, doc.TargetsNone)
	}
	if len(doc.Targets) != 0 {
		t.Fatalf("Targets = %v, want none", doc.Targets)
	}
}

func TestParseHTMLRejectsInvalidTargets(t *testing.T) {
	for _, value := range []string{"", "bluesky", "none,bluesky", "unknown"} {
		t.Run(value, func(t *testing.T) {
			input := `<html><head><title>Invalid</title><meta name="pdg:targets" content="` + value + `"></head><body><article>Content</article></body></html>`
			if _, err := ParseHTML(strings.NewReader(input)); err == nil {
				t.Fatal("ParseHTML() unexpectedly succeeded")
			}
		})
	}
}
