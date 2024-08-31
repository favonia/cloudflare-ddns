package monitor

import (
	"fmt"
	"slices"
	"strings"
)

// Message holds the messages and success/failure status for monitors.
type Message struct {
	OK    bool
	Lines []string
}

// NewMessage creates a new empty Message.
func NewMessage() Message { return Message{OK: true, Lines: nil} }

// NewMessagef creates a new Message containing one formatted line.
func NewMessagef(ok bool, format string, args ...any) Message {
	return Message{OK: ok, Lines: []string{fmt.Sprintf(format, args...)}}
}

// Format turns the message into a single string.
func (m Message) Format() string { return strings.Join(m.Lines, "\n") }

// MergeMessages keeps only the ones with highest severity.
func MergeMessages(msgs ...Message) Message {
	var (
		OK    = true
		Lines = map[bool][][]string{}
	)

	for _, msg := range msgs {
		OK = OK && msg.OK
		Lines[msg.OK] = append(Lines[msg.OK], msg.Lines)
	}

	return Message{
		OK:    OK,
		Lines: slices.Concat(Lines[OK]...),
	}
}

// IsEmpty checks if the message is empty.
func (m Message) IsEmpty() bool { return len(m.Lines) == 0 }
