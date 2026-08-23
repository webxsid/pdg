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
