// Package file virtualizes file systems for mock testing.
package file

import (
	"bytes"
	"io/fs"
	"iter"
	"os"
	pathpkg "path"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// fileSystem represents the file system in use. By default, it points to the actual file system,
// and can be modified to a virtual file system for testing.
var fileSystem fs.FS = os.DirFS("/") //nolint:gochecknoglobals

// SetFSForTesting replaces the backing file system used by this package.
// It exists to support tests in dependent packages.
func SetFSForTesting(vfs fs.FS) {
	fileSystem = vfs
}

// ResetFSForTesting restores the default backing file system after tests.
func ResetFSForTesting() {
	fileSystem = os.DirFS("/")
}

// processPath validates that path starts with "/" and converts it to a relative form for [fileSystem].
// If the path does not start with "/", fixedPath holds the suggested correction ("/"+path)
// and ok is false. If it starts with "/", relPath is for use with [fileSystem] and fixedPath equals path.
//
// This uses [path.Clean] (forward-slash only) instead of filepath.IsAbs and filepath.Rel
// so that the behavior is OS-independent: the app targets Linux (Docker), and all valid
// paths start with "/". Using the path package also lets tests run without a //go:build unix tag.
func processPath(ppfmt pp.PP, path string) (relPath string, fixedPath string, ok bool) {
	if !strings.HasPrefix(path, "/") {
		ppfmt.Noticef(pp.EmojiUserError,
			"The path %s is not absolute; to use an absolute path, prefix it with /",
			pp.QuoteIfUnsafeInSentence(path))
		return "", "/" + path, false
	}

	// pathpkg.Clean normalizes redundant slashes, dot segments, and the POSIX-permitted
	// "//path" prefix (which Linux treats identically to "/path") into a canonical form.
	// os.DirFS(...).Open() does not accept absolute paths, so strip the leading "/".
	return pathpkg.Clean(path)[1:], path, true
}

// RequireAbsolutePath checks that path is absolute. On failure, it prints a generic error
// and returns the suggested fix ("/"+path). Callers may use fixedPath for context-specific hints.
func RequireAbsolutePath(ppfmt pp.PP, path string) (fixedPath string, ok bool) {
	_, fixedPath, ok = processPath(ppfmt, path)
	return fixedPath, ok
}

// ReadString reads the content of the file at path.
// The path must be absolute; relative paths are rejected.
func ReadString(ppfmt pp.PP, path string) (string, bool) {
	relPath, _, ok := processPath(ppfmt, path)
	if !ok {
		return "", false
	}

	body, err := fs.ReadFile(fileSystem, relPath)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to read %s: %v", pp.QuoteIfUnsafeInSentence(path), err)
		return "", false
	}

	return string(bytes.TrimSpace(body)), true
}

// ProcessLines returns an iterator over non-blank, non-comment lines in content.
// Each yielded pair is (1-based line number, trimmed content after stripping # comments).
func ProcessLines(content string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		lineNum := 0
		for line := range strings.Lines(content) {
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
	}
}

// ReadLines reads a file and returns an iterator over its non-blank, non-comment lines.
// Each yielded pair is (1-based line number, trimmed content after stripping # comments).
// The path must be absolute; relative paths are rejected.
// If the file cannot be read, lines is nil and ok is false.
func ReadLines(ppfmt pp.PP, path string) (lines iter.Seq2[int, string], ok bool) {
	relPath, _, ok := processPath(ppfmt, path)
	if !ok {
		return nil, false
	}

	body, err := fs.ReadFile(fileSystem, relPath)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "Failed to read %s: %v", pp.QuoteIfUnsafeInSentence(path), err)
		return nil, false
	}

	return ProcessLines(string(body)), true
}
