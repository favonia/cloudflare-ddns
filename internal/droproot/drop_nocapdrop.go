//go:build nocapdrop

package droproot

import "github.com/favonia/cloudflare-ddns/internal/pp"

func DropPrivileges(ppfmt pp.PP) bool {
	// No-Op for compat
	return true
}
