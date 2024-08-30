package message

import (
	"fmt"
	"slices"
	"strings"
)

// NotifierMessage holds the messages and success/failure status for notifiers.
type NotifierMessage []string

// NewNotifierMessage creates a new empty NotifierMessage.
func NewNotifierMessage() NotifierMessage { return nil }

// NewNotifierMessagef creates a new MonitorMessage containing one formatted string.
func NewNotifierMessagef(format string, args ...any) NotifierMessage {
	return NotifierMessage{fmt.Sprintf(format, args...)}
}

// MergeNotifierMessages keeps only the ones with highest severity.
func MergeNotifierMessages(msgs ...NotifierMessage) NotifierMessage {
	return slices.Concat[NotifierMessage, string](msgs...)
}

// Format turns the message into a single string.
func (m NotifierMessage) Format() string { return strings.Join(m, " ") }

// IsEmpty checks if the message is empty.
func (m NotifierMessage) IsEmpty() bool { return len(m) == 0 }
