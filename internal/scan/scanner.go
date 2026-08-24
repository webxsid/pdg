package scan

import (
	"context"
	"io/fs"
	"path"
	"strings"

	"github.com/webxsid/pdg/internal/shared"
)

type Scanner struct{}

type SkippedFile struct {
	Path   string
	Reason string
}

type Result struct {
	Documents []Document
	Skipped   []SkippedFile
}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) Scan(
	ctx context.Context,
	filesystem fs.FS,
) (Result, error) {
	result := Result{}

	err := fs.WalkDir(
		filesystem,
		".",
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if entry.IsDir() {
				return nil
			}

			if !isHTML(path) {
				return nil
			}

			file, err := filesystem.Open(path)
			if err != nil {
				result.Skipped = append(result.Skipped, SkippedFile{Path: path, Reason: "failed to open file: " + err.Error()})
				return nil
			}
			defer shared.SafeCloseFsFile(file)

			doc, err := ParseHTML(file)
			if err != nil {
				result.Skipped = append(result.Skipped, SkippedFile{Path: path, Reason: "failed to parse HTML: " + err.Error()})
				return nil
			}

			doc.Path = path
			result.Documents = append(result.Documents, doc)
			return nil
		},
	)
	if err != nil {
		return Result{}, err
	}

	return result, nil
}

func isHTML(filePath string) bool {
	return strings.EqualFold(path.Ext(filePath), ".html") || strings.EqualFold(path.Ext(filePath), ".htm")
}
