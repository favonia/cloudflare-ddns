package api

import (
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// HandleOwnershipPolicy bundles the handle-bound ownership selectors and
// shutdown authority that affect cache correctness and WAF cleanup behavior.
type HandleOwnershipPolicy struct {
	ManagedRecordsCommentRegex        *regexp.Regexp
	ManagedWAFListItemsCommentRegex   *regexp.Regexp
	AllowWholeWAFListDeleteOnShutdown bool
}

// MatchManagedRecordComment reports whether a DNS record comment is in scope.
func (p HandleOwnershipPolicy) MatchManagedRecordComment(comment string) bool {
	if p.ManagedRecordsCommentRegex == nil {
		return true
	}
	return p.ManagedRecordsCommentRegex.MatchString(comment)
}

// MatchManagedWAFListItemComment reports whether a WAF item comment is in scope.
func (p HandleOwnershipPolicy) MatchManagedWAFListItemComment(comment string) bool {
	if p.ManagedWAFListItemsCommentRegex == nil {
		return true
	}
	return p.ManagedWAFListItemsCommentRegex.MatchString(comment)
}

// Sanitize normalizes contradictory ownership settings and logs advisories.
func (p HandleOwnershipPolicy) Sanitize(ppfmt pp.PP) HandleOwnershipPolicy {
	if !p.AllowWholeWAFListDeleteOnShutdown {
		return p
	}

	// Whole-list final deletion is allowed only when WAF item ownership uses the
	// empty default selector. A nil selector is treated the same as that default.
	if p.ManagedWAFListItemsCommentRegex == nil || p.ManagedWAFListItemsCommentRegex.String() == "" {
		return p
	}

	ppfmt.Noticef(pp.EmojiUserWarning,
		"DELETE_ON_STOP is enabled, but "+
			"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s) is non-empty; "+
			"the updater will keep the list and delete only items managed by this updater",
		pp.QuotePreview(p.ManagedWAFListItemsCommentRegex.String(), advisoryValuePreviewLimit),
	)
	p.AllowWholeWAFListDeleteOnShutdown = false
	return p
}
