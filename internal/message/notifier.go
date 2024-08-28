package message

import (
	"slices"
	"strings"
)

// NotifierMessage holds the messages and success/failure status for notifiers.
type NotifierMessage []string

// NewNotifierMessage creates a new empty NotifierMessage.
func NewNotifierMessage() NotifierMessage { return nil }

// MergeNotifierMessages keeps only the ones with highest severity.
func MergeNotifierMessages(msgs ...NotifierMessage) NotifierMessage {
	return slices.Concat[NotifierMessage, string](msgs...)
}

// Format turns the message into a single string.
func (m NotifierMessage) Format() string {
	return strings.Join(m, " ")
}
