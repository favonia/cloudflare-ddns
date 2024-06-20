//go:build !linux || nocapdrop

// Package droproot drops root privileges.
package droproot

import (
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func tryRaiseCapabilitySETUID() {}
func tryRaiseCapabilitySETGID() {}
func dropCapabilities(ppfmt pp.PP) bool {
	ppfmt.Infof(pp.EmojiDisabled, "Support of Linux capabilities was disabled")
	return true
}
