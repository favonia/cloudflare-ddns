package updater

import (
	"fmt"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
)

// Message carries the updater work result and messages for heartbeat services and notifiers.
type Message struct {
	HeartbeatMessage heartbeat.Message
	NotifierMessage  notifier.Message
	NotificationKind notifier.Kind

	// Semantic entries own their slices and retain encounter order independently
	// of heartbeat/notifier grouping. A problem never invents a DNS outcome.
	dns      []dnsOutcome
	waf      []wafOutcome
	problems []operationProblem
	cleanup  bool
}

// Failed reports whether updater work had an unrecovered failure.
// Accepted asynchronous work is not a failure. Reporter delivery is separate.
func (m Message) Failed() bool {
	return !m.HeartbeatMessage.OK
}

// Notification returns the notifier-facing message with its classification.
func (m Message) Notification() notifier.Notification {
	return notifier.NewNotification(m.NotificationKind, m.NotifierMessage)
}

// newMessage creates a new, empty message.
func newMessage() Message {
	return Message{
		HeartbeatMessage: heartbeat.NewMessage(),
		NotifierMessage:  notifier.NewMessage(),
		NotificationKind: "",
		dns:              nil,
		waf:              nil,
		problems:         nil,
		cleanup:          false,
	}
}

// mergeMessages combines multiple compound messages.
func mergeMessages(msgs ...Message) Message {
	hms := make([]heartbeat.Message, len(msgs))
	nms := make([]notifier.Message, len(msgs))
	merged := newMessage()

	for i, msg := range msgs {
		hms[i] = msg.HeartbeatMessage
		nms[i] = msg.NotifierMessage
		for _, outcome := range msg.dns {
			merged.dns = append(merged.dns, outcome.clone())
		}
		merged.waf = append(merged.waf, msg.waf...)
		for _, problem := range msg.problems {
			problem.domains = slices.Clone(problem.domains)
			merged.problems = append(merged.problems, problem)
		}
		merged.cleanup = merged.cleanup || msg.cleanup
	}

	merged.HeartbeatMessage = heartbeat.MergeMessages(hms...)
	merged.NotifierMessage = notifier.MergeMessages(nms...)
	return merged
}

func classifyNotification(msg Message, successKind, failureKind notifier.Kind) Message {
	if msg.HeartbeatMessage.OK {
		msg.NotificationKind = successKind
	} else {
		msg.NotificationKind = failureKind
	}
	return msg
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
