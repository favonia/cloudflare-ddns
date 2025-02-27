package pp

// QueuedPP is a pretty printer that queues all printing operations
// (but executes non-printing operations immediately).
// [QueuedPP.Flush] will then execute all queued printing operations.
//
// QueuePP itself is not goroute-safe, but can be used to queue printing
// operations from different goroutines to achieve goroutine-safety.
type QueuedPP struct {
	upstream PP
	queue    *[]func()
}

// NewQueued creates a new pretty printer that queues all printing operations.
// It is assumed that [PP.IsShowing] and [PP.Indent] do not induce race conditions.
func NewQueued(pp PP) QueuedPP {
	var empty []func()
	return QueuedPP{upstream: pp, queue: &empty}
}

// IsShowing calls [PP.IsShowing] of the upstream.
func (b QueuedPP) IsShowing(v Verbosity) bool {
	return b.upstream.IsShowing(v)
}

// Indent calls [PP.Indent] and return a new batch printer with a new upstream.
// The call queue is shared with the original batch printer.
func (b QueuedPP) Indent() PP {
	b.upstream = b.upstream.Indent()
	return b
}

// BlankLineIfVerbose queues a call to [PP.BlankLineIfVerbose] of the upstream.
func (b QueuedPP) BlankLineIfVerbose() {
	// It's important to save the current upstream because [Indent] may change it.
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.BlankLineIfVerbose() })
}

// Infof queues a call to [PP.Infof] of the upstream.
func (b QueuedPP) Infof(emoji Emoji, format string, args ...any) {
	// It's important to save the current upstream because [Indent] may change it.
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.Infof(emoji, format, args...) })
}

// Noticef queues a call to [PP.Noticef] of the upstream.
func (b QueuedPP) Noticef(emoji Emoji, format string, args ...any) {
	// It's important to save the current upstream because [Indent] may change it.
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.Noticef(emoji, format, args...) })
}

// Suppress queues a call to [PP.Suppress] of the upstream.
func (b QueuedPP) Suppress(id ID) {
	// It's important to save the current upstream because [Indent] may change it.
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.Suppress(id) })
}

// InfoOncef queues a call to [PP.InfoOncef] of the upstream.
func (b QueuedPP) InfoOncef(id ID, emoji Emoji, format string, args ...any) {
	// It's important to save the current upstream because [Indent] may change it.
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.InfoOncef(id, emoji, format, args...) })
}

// NoticeOncef queues a call to [PP.NoticeOncef] of the upstream.
func (b QueuedPP) NoticeOncef(id ID, emoji Emoji, format string, args ...any) {
	// It's important to save the current upstream because [Indent] may change it.
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.NoticeOncef(id, emoji, format, args...) })
}

// Flush executes all queued function calls.
func (b QueuedPP) Flush() {
	for _, f := range *b.queue {
		f()
	}
	*b.queue = nil
}
