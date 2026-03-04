package updater

import (
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

// Message encapsulates the messages to both heartbeat services and notifiers.
type Message struct {
	HeartbeatMessage heartbeat.Message
	NotifierMessage  notifier.Message
}

// NewMessage creates a new, empty message.
func NewMessage() Message {
	return Message{
		HeartbeatMessage: heartbeat.NewMessage(),
		NotifierMessage:  notifier.NewMessage(),
	}
}

// MergeMessages combines multiple compound messages.
func MergeMessages(msgs ...Message) Message {
	hms := make([]heartbeat.Message, len(msgs))
	nms := make([]notifier.Message, len(msgs))

	for i := range msgs {
		hms[i] = msgs[i].HeartbeatMessage
		nms[i] = msgs[i].NotifierMessage
	}

	return Message{
		HeartbeatMessage: heartbeat.MergeMessages(hms...),
		NotifierMessage:  notifier.MergeMessages(nms...),
	}
}
