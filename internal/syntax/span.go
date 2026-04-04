package syntax

// Span identifies a byte range [Start, End) in the source input.
type Span struct {
	// Start is the inclusive start byte offset.
	Start int
	// End is the exclusive end byte offset.
	End int
}

// fromTo returns the span from start.Start through end.End.
func fromTo(start, end Span) Span {
	return Span{Start: start.Start, End: end.End}
}
