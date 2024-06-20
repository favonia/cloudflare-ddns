//go:build !nocapdrop

// Package droproot drops root privileges.
package droproot

import (
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// tryRaiseCapability attempts to raise the capability val.
//
// The newly gained capability (if any) will be dropped by dropCapabilities later.
// We have this function because, in some strange cases, the user might have the
// capability to raise certain capabilities to (ironically) drop more capabilities.
// In any case, it doesn't hurt to try!
func tryRaiseCapability(val cap.Value) {
	c, err := cap.GetPID(0)
	if err != nil {
		return
	}

	if err := c.SetFlag(cap.Effective, true, val); err != nil {
		return
	}

	_ = c.SetProc()
}
func tryRaiseCapabilitySETUID() { tryRaiseCapability(cap.SETUID) }
func tryRaiseCapabilitySETGID() { tryRaiseCapability(cap.SETGID) }

// dropCapabilities drop all capabilities as the last step.
func dropCapabilities(ppfmt pp.PP) bool {
	_ = cap.NewSet().SetProc()
	checkCapabilities(ppfmt)

	return true
}

func checkCapabilities(ppfmt pp.PP) bool {
	now := cap.GetProc()
	diff, err := now.Cf(cap.NewSet())
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiImpossible, "Failed to check Linux capabilities: %v", err)
		return false
	case diff != 0:
		ppfmt.Noticef(pp.EmojiWarning, "Failed to drop all Linux capabilities; current ones: %v", now)
		return false
	default:
		return true
	}
}
