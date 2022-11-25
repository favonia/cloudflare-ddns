// Package file virtualizes file systems for mock testing.
package file

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// LinuxRoot is the root in Linux.
const LinuxRoot string = "/"

// FS represents the file system in use. By default, it points to the actual file system,
// and can be modified to a virtual file system for testing.
var FS = os.DirFS(LinuxRoot) //nolint:gochecknoglobals

// ReadString reads the content of the file at path. It treats an absolute path as
// a path relative to the root of [FS].
func ReadString(ppfmt pp.PP, path string) (string, bool) {
	// os.DirFS(...).Open() does not accept absolute paths
	if filepath.IsAbs(path) {
		newpath, err := filepath.Rel(LinuxRoot, path)
		if err != nil {
			ppfmt.Errorf(pp.EmojiImpossible, `%q is an absolute path but does not start with %q: %v`, path, LinuxRoot, err)
			return "", false
		}
		path = newpath
	}

	body, err := fs.ReadFile(FS, path)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(body)), true
}
