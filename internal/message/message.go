// Package message defines the structures holding messages for
// monitors and notifiers.
package message

import (
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

// Message encapsulates the messages to both monitiors and notifiers.
type Message struct {
	MonitorMessage  monitor.Message
	NotifierMessage notifier.Message
}

// New creates a new, empty message.
func New() Message {
	return Message{
		MonitorMessage:  monitor.NewMessage(),
		NotifierMessage: notifier.NewMessage(),
	}
}

// Merge combines multiple compound messages.
func Merge(msgs ...Message) Message {
	mms := make([]monitor.Message, len(msgs))
	nms := make([]notifier.Message, len(msgs))

	for i := range msgs {
		mms[i] = msgs[i].MonitorMessage
		nms[i] = msgs[i].NotifierMessage
	}

	return Message{
		MonitorMessage:  monitor.MergeMessages(mms...),
		NotifierMessage: notifier.MergeMessages(nms...),
	}
}
