package message

import (
	"fmt"
	"slices"
	"strings"
)

// MonitorMessage holds the messages and success/failure status for monitors.
type MonitorMessage struct {
	OK    bool
	Lines []string
}

// NewMonitorMessage creates a new empty MonitorMessage.
func NewMonitorMessage() MonitorMessage { return MonitorMessage{OK: true, Lines: nil} }

// NewMonitorMessagef creates a new MonitorMessage containing one formatted line.
func NewMonitorMessagef(ok bool, format string, args ...any) MonitorMessage {
	return MonitorMessage{OK: ok, Lines: []string{fmt.Sprintf(format, args...)}}
}

// Format turns the message into a single string.
func (m MonitorMessage) Format() string { return strings.Join(m.Lines, "\n") }

// MergeMonitorMessages keeps only the ones with highest severity.
func MergeMonitorMessages(msgs ...MonitorMessage) MonitorMessage {
	var (
		OK    = true
		Lines = map[bool][][]string{}
	)

	for _, msg := range msgs {
		OK = OK && msg.OK
		Lines[msg.OK] = append(Lines[msg.OK], msg.Lines)
	}

	return MonitorMessage{
		OK:    OK,
		Lines: slices.Concat(Lines[OK]...),
	}
}

// IsEmpty checks if the message is empty.
func (m MonitorMessage) IsEmpty() bool { return len(m.Lines) == 0 }
