// Package file virtualizes file systems for mock testing.
package file

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// LinuxRoot is the root in Linux.
const LinuxRoot string = "/"

// FS represents the file system in use. By default, it points to the actual file system,
// and can be modified to a virtual file system for testing.
var FS = os.DirFS(LinuxRoot) //nolint:gochecknoglobals

// errNotAbsolute is returned by processPath when the path is not absolute.
var errNotAbsolute = errors.New("path is not absolute")

// processPath validates that path is absolute and converts it to a relative form for [FS].
// If the path is not absolute, fixedPath holds the suggested correction ("/"+path)
// and the error is [errNotAbsolute]. If absolute, relPath is for use with [FS]
// and fixedPath equals path. No messaging — callers handle error reporting.
func processPath(path string) (relPath string, fixedPath string, err error) {
	if !filepath.IsAbs(path) {
		return "", "/" + path, errNotAbsolute
	}

	// os.DirFS(...).Open() does not accept absolute paths
	relPath, err = filepath.Rel(LinuxRoot, path)
	if err != nil {
		return "", path, fmt.Errorf("%q is an absolute path but does not start with %q: %w", path, LinuxRoot, err)
	}

	return relPath, path, nil
}

// RequireAbsolutePath checks that path is absolute. On failure, it prints a generic error
// and returns the suggested fix ("/"+path). Callers may use fixedPath for context-specific hints.
func RequireAbsolutePath(ppfmt pp.PP, path string) (fixedPath string, ok bool) {
	_, fixedPath, err := processPath(path)
	if err != nil {
		if errors.Is(err, errNotAbsolute) {
			ppfmt.Noticef(pp.EmojiUserError,
				"The path %q is not absolute; to use an absolute path, prefix it with /", path)
		} else {
			ppfmt.Noticef(pp.EmojiImpossible, "%v", err)
		}
		return fixedPath, false
	}
	return fixedPath, true
}

// ReadString reads the content of the file at path.
// The path must be absolute; relative paths are rejected.
func ReadString(ppfmt pp.PP, path string) (string, bool) {
	relPath, _, err := processPath(path)
	if err != nil {
		if errors.Is(err, errNotAbsolute) {
			ppfmt.Noticef(pp.EmojiUserError,
				"The path %q is not absolute; to use an absolute path, prefix it with /", path)
		} else {
			ppfmt.Noticef(pp.EmojiImpossible, "%v", err)
		}
		return "", false
	}

	body, err := fs.ReadFile(FS, relPath)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(body)), true
}

// ReadLines reads a file and returns an iterator over its non-blank, non-comment lines.
// Each yielded pair is (1-based line number, trimmed content after stripping # comments).
// The path must be absolute; relative paths are rejected.
// If the file cannot be read, lines is nil and ok is false.
func ReadLines(ppfmt pp.PP, path string) (lines iter.Seq2[int, string], ok bool) {
	relPath, _, err := processPath(path)
	if err != nil {
		if errors.Is(err, errNotAbsolute) {
			ppfmt.Noticef(pp.EmojiUserError,
				"The path %q is not absolute; to use an absolute path, prefix it with /", path)
		} else {
			ppfmt.Noticef(pp.EmojiImpossible, "%v", err)
		}
		return nil, false
	}

	body, err := fs.ReadFile(FS, relPath)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return nil, false
	}

	return func(yield func(int, string) bool) {
		lineNum := 0
		for line := range strings.Lines(string(body)) {
			lineNum++

			// Strip comments.
			if i := strings.IndexByte(line, '#'); i >= 0 {
				line = line[:i]
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !yield(lineNum, line) {
				return
			}
		}
	}, true
}
