package updater

import (
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

// Message encapsulates the messages to both monitiors and notifiers.
type Message struct {
	MonitorMessage  monitor.Message
	NotifierMessage notifier.Message
}

// NewMessage creates a new, empty message.
func NewMessage() Message {
	return Message{
		MonitorMessage:  monitor.NewMessage(),
		NotifierMessage: notifier.NewMessage(),
	}
}

// MergeMessages combines multiple compound messages.
func MergeMessages(msgs ...Message) Message {
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
