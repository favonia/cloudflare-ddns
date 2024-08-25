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

// Infof queues a call to [PP.Infof] of the upstream.
func (b QueuedPP) Infof(emoji Emoji, format string, args ...any) {
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.Infof(emoji, format, args...) })
}

// Noticef queues a call to [PP.Noticef] of the upstream.
func (b QueuedPP) Noticef(emoji Emoji, format string, args ...any) {
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.Noticef(emoji, format, args...) })
}

// SuppressHint queues a call to [PP.SuppressHint] of the upstream.
func (b QueuedPP) SuppressHint(hint Hint) {
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.SuppressHint(hint) })
}

// Hintf queues a call to [PP.Hintf] of the upstream.
func (b QueuedPP) Hintf(hint Hint, format string, args ...any) {
	upstream := b.upstream
	*b.queue = append(*b.queue, func() { upstream.Hintf(hint, format, args...) })
}

// Flush executes all queued function calls.
func (b QueuedPP) Flush() {
	for _, f := range *b.queue {
		f()
	}
	*b.queue = nil
}
