package file

import (
	"bytes"

	"github.com/spf13/afero"

	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

var FS = afero.NewOsFs() //nolint:gochecknoglobals

func ReadString(indent pp.Indent, path string) (string, bool) {
	body, err := afero.ReadFile(FS, path)
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(body)), true
}
