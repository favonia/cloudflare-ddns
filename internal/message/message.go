// Package message defines the structure holding messages to
// monitors and notifiers.
package message

// Message holds the messages and success/failure status.
// To monitors, the status is far more important than the message,
// and to notifiers, all messages are important.
type Message struct {
	OK               bool
	MonitorMessages  []string
	NotifierMessages []string
}

// NewEmpty creates a new empty Message.
func NewEmpty() Message {
	return Message{
		OK:               true,
		MonitorMessages:  nil,
		NotifierMessages: nil,
	}
}

// Merge merges a list of messages in the following way:
// - For monitors, we collect only the messages of higher severity.
// - For notifiers, we collect all the messages.
func Merge(msgs ...Message) Message {
	var (
		allOK                        = true
		allMonitorMessages           = map[bool][]string{}
		allNotifierMessages []string = nil
	)

	for _, msg := range msgs {
		allOK = allOK && msg.OK
		allMonitorMessages[msg.OK] = append(allMonitorMessages[msg.OK], msg.MonitorMessages...)
		allNotifierMessages = append(allNotifierMessages, msg.NotifierMessages...)
	}

	return Message{
		OK:               allOK,
		MonitorMessages:  allMonitorMessages[allOK],
		NotifierMessages: allNotifierMessages,
	}
}
