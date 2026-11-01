package updater

import (
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
)

// Message carries the updater work result and heartbeat status.
// ReportingState assesses its semantic outcomes for notification composition.
type Message struct {
	HeartbeatMessage heartbeat.Message

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

// newMessage creates a new, empty message.
func newMessage() Message {
	return Message{
		HeartbeatMessage: heartbeat.NewMessage(),
		dns:              nil,
		waf:              nil,
		problems:         nil,
		cleanup:          false,
	}
}

// mergeMessages combines multiple compound messages.
func mergeMessages(msgs ...Message) Message {
	hms := make([]heartbeat.Message, len(msgs))
	merged := newMessage()

	for i, msg := range msgs {
		hms[i] = msg.HeartbeatMessage
		for _, outcome := range msg.dns {
			merged.dns = append(merged.dns, outcome.clone())
		}
		for _, outcome := range msg.waf {
			merged.waf = append(merged.waf, outcome.clone())
		}
		for _, problem := range msg.problems {
			problem.domains = slices.Clone(problem.domains)
			merged.problems = append(merged.problems, problem)
		}
		merged.cleanup = merged.cleanup || msg.cleanup
	}

	merged.HeartbeatMessage = heartbeat.MergeMessages(hms...)
	return merged
}
