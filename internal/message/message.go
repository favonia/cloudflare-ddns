// Package message defines the structure holding messages to
// monitors and notifiers.
package message

// Message holds the messages and success/failure status.
// To monitors, the status is far more important than the message,
// and to notifiers, all messages are important.
type Message struct {
	Ok               bool
	MonitorMessages  []string
	NotifierMessages []string
}

// NewEmpty creates a new empty Message.
func NewEmpty() Message {
	return Message{
		Ok:               true,
		MonitorMessages:  nil,
		NotifierMessages: nil,
	}
}

// Merge merges a list of messages in the following way:
// - For monitors, we collect only the messages of higher severity.
// - For notifiers, we collect all the messages.
func Merge(msgs ...Message) Message {
	var (
		allOk                        = true
		allMonitorMessages           = map[bool][]string{}
		allNotifierMessages []string = nil
	)

	for _, msg := range msgs {
		allOk = allOk && msg.Ok
		allMonitorMessages[msg.Ok] = append(allMonitorMessages[msg.Ok], msg.MonitorMessages...)
		allNotifierMessages = append(allNotifierMessages, msg.NotifierMessages...)
	}

	return Message{
		Ok:               allOk,
		MonitorMessages:  allMonitorMessages[allOk],
		NotifierMessages: allNotifierMessages,
	}
}
