package provider

import (
	"path/filepath"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewFile creates a [protocol.File] provider that reads IPs from a file on every detection cycle.
// The path must be absolute.
func NewFile(ppfmt pp.PP, key string, path string) (Provider, bool) {
	if !filepath.IsAbs(path) {
		ppfmt.Noticef(pp.EmojiUserError,
			"The path %q in %s is not absolute; use an absolute path", path, key)
		return nil, false
	}

	return protocol.NewFile("file:"+path, path), true
}

// MustNewFile creates a [protocol.File] provider and panics if the path is not absolute.
func MustNewFile(path string) Provider {
	var buf strings.Builder
	p, ok := NewFile(pp.NewDefault(&buf), "IP_PROVIDER", path)
	if !ok {
		panic(buf.String())
	}
	return p
}
