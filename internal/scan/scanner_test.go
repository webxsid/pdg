package scan

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestScanner(t *testing.T) {
	filesystem := fstest.MapFS{
		"index.html": {
			Data: []byte(`
				<html>
					<head>
						<title>Home</title>
						<link
							rel="canonical"
							href="https://example.com"
						>
					</head>
				</html>
			`),
		},

		"blog/hello/index.html": {
			Data: []byte(`
				<html>
					<head>
						<title>Hello Atmosphere</title>

						<link
							rel="canonical"
							href="https://example.com/blog/hello"
						>

						<meta
							property="article:published_time"
							content="2026-08-23T08:00:00Z"
						>
					</head>

					<body>
						<article>
							<h1>Hello</h1>
							<p>Welcome to the Atmosphere.</p>
						</article>
					</body>
				</html>
			`),
		},
	}

	scanner := NewScanner()

	result, err := scanner.Scan(
		context.Background(),
		filesystem,
	)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Documents) != 1 {
		t.Fatalf(
			"Documents = %d, want 1",
			len(result.Documents),
		)
	}

	if len(result.Skipped) != 1 {
		t.Fatalf(
			"Skipped = %d, want 1",
			len(result.Skipped),
		)
	}

	document := result.Documents[0]

	if document.Title != "Hello Atmosphere" {
		t.Errorf(
			"Title = %q, want %q",
			document.Title,
			"Hello Atmosphere",
		)
	}

	if document.Path != "blog/hello/index.html" {
		t.Errorf(
			"Path = %q",
			document.Path,
		)
	}
}
