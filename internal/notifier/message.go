package notifier

import (
	"fmt"
	"slices"
	"strings"
)

// Message holds the messages and success/failure status for notifiers.
type Message []string

// NewMessage creates a new empty Message.
func NewMessage() Message { return nil }

// NewMessagef creates a new MonitorMessage containing one formatted string.
func NewMessagef(format string, args ...any) Message {
	return Message{fmt.Sprintf(format, args...)}
}

// MergeMessages keeps only the ones with highest severity.
func MergeMessages(msgs ...Message) Message {
	return slices.Concat[Message, string](msgs...)
}

// Format turns the message into a single string.
func (m Message) Format() string { return strings.Join(m, " ") }

// IsEmpty checks if the message is empty.
func (m Message) IsEmpty() bool { return len(m) == 0 }
