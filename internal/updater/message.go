package updater

import (
	"fmt"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

// Message encapsulates the messages to both heartbeat services and notifiers.
type Message struct {
	HeartbeatMessage heartbeat.Message
	NotifierMessage  notifier.Message
}

// newMessage creates a new, empty message.
func newMessage() Message {
	return Message{
		HeartbeatMessage: heartbeat.NewMessage(),
		NotifierMessage:  notifier.NewMessage(),
	}
}

// mergeMessages combines multiple compound messages.
func mergeMessages(msgs ...Message) Message {
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

// appendNotifierFragmentf appends a notifier fragment while keeping the
// sentence-initial and sentence-continuation wording explicit at the call site.
func appendNotifierFragmentf(fragments []string, initialFormat string, continuedFormat string, args ...any) []string {
	if len(fragments) == 0 {
		return append(fragments, fmt.Sprintf(initialFormat, args...))
	}

	return append(fragments, fmt.Sprintf(continuedFormat, args...))
}

func finishNotifierMessage(fragments []string) notifier.Message {
	if len(fragments) == 0 {
		return nil
	}

	fragments = append(fragments, ".")
	return notifier.Message{strings.Join(fragments, "")}
}
