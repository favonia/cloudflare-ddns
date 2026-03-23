// Package file virtualizes file systems for mock testing.
package file

import (
	"bytes"
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

// toRelPath converts an absolute path to a path relative to [LinuxRoot] for use with [FS].
// It rejects non-absolute paths with an actionable error because the updater targets
// Docker and systemd environments where the working directory is arbitrary.
func toRelPath(ppfmt pp.PP, path string) (string, bool) {
	if !filepath.IsAbs(path) {
		ppfmt.Noticef(pp.EmojiUserError, "The path %q is not absolute; use an absolute path", path)
		return "", false
	}

	// os.DirFS(...).Open() does not accept absolute paths
	relPath, err := filepath.Rel(LinuxRoot, path)
	if err != nil {
		ppfmt.Noticef(pp.EmojiImpossible, `%q is an absolute path but does not start with %q: %v`, path, LinuxRoot, err)
		return "", false
	}

	return relPath, true
}

// ReadString reads the content of the file at path.
// The path must be absolute; relative paths are rejected.
func ReadString(ppfmt pp.PP, path string) (string, bool) {
	relPath, ok := toRelPath(ppfmt, path)
	if !ok {
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
	relPath, ok := toRelPath(ppfmt, path)
	if !ok {
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
