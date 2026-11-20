package notifier

import (
	"fmt"
	"strings"
)

// Message holds the text sent to notifiers.
type Message []string

// NewMessage creates a new empty Message.
func NewMessage() Message { return nil }

// NewMessagef creates a new Message containing one formatted string.
func NewMessagef(format string, args ...any) Message {
	return Message{fmt.Sprintf(format, args...)}
}

// Format turns the message into a single string.
func (m Message) Format() string { return strings.Join(m, " ") }

// IsEmpty checks if the message is empty.
func (m Message) IsEmpty() bool { return len(m) == 0 }
