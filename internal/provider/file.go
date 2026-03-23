package provider

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

// NewFile creates a [protocol.File] provider that reads IPs from a file on every detection cycle.
// The path must be absolute.
func NewFile(ppfmt pp.PP, key string, path string) (Provider, bool) {
	fixedPath, ok := file.RequireAbsolutePath(ppfmt, path)
	if !ok {
		ppfmt.Noticef(pp.EmojiHint, "Try setting %s=file:%s", key, fixedPath)
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
