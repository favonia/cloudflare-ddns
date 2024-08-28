// Package message defines the structures holding messages for
// monitors and notifiers.
package message

// Message encapsulates the messages to both monitiors and notifiers.
type Message struct {
	NotifierMessage
	MonitorMessage
}

// New creates a new, empty message.
func New() Message {
	return Message{
		MonitorMessage:  NewMonitorMessage(),
		NotifierMessage: NewNotifierMessage(),
	}
}

// Merge combines multiple compound messages.
func Merge(msgs ...Message) Message {
	mms := make([]MonitorMessage, len(msgs))
	nms := make([]NotifierMessage, len(msgs))

	for i := range msgs {
		mms[i] = msgs[i].MonitorMessage
		nms[i] = msgs[i].NotifierMessage
	}

	return Message{
		MonitorMessage:  MergeMonitorMessages(mms...),
		NotifierMessage: MergeNotifierMessages(nms...),
	}
}
