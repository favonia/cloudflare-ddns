package file

import (
	"bytes"
	"io/fs"
	"os"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var FS = os.DirFS("/") //nolint:gochecknoglobals

func ReadString(ppfmt pp.Fmt, path string) (string, bool) {
	body, err := fs.ReadFile(FS, path)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(body)), true
}
