package file

import (
	"bytes"
	"io"
	"os"

	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

func ReadString(indent pp.Indent, path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to open %q: %v", path, err)
		return "", false
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to read %q: %v", path, err)
		return "", false
	}

	return string(bytes.TrimSpace(content)), true
}
