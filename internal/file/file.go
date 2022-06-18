package file

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var FS = os.DirFS("/") //nolint:gochecknoglobals

func ReadString(ppfmt pp.PP, path string) (string, bool) {
	// os.DirFS(...).Open() does not accept absolute paths
	if filepath.IsAbs(path) {
		newpath, err := filepath.Rel("/", path)
		if err != nil {
			ppfmt.Errorf(pp.EmojiImpossible, `%q is an absolute path but does not start with "/": %v`, err)
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
