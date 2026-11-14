package updater

import (
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// Message carries the updater work result and heartbeat status.
// ReportOnce and ScheduledReporter report its semantic outcomes.
type Message struct {
	// HeartbeatMessage is a derived delivery projection. Changing it does not
	// change the retained work result or notification assessment.
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
	if len(m.problems) > 0 {
		return true
	}
	for _, outcome := range m.dns {
		if outcome.result.Code == setter.ResponseFailed {
			return true
		}
	}
	for _, outcome := range m.waf {
		if outcome.result.Code == setter.ResponseFailed {
			return true
		}
	}
	return false
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
	merged := newMessage()

	for _, msg := range msgs {
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

	return merged.withHeartbeat()
}

// withHeartbeat derives delivery content from the completed semantic result.
// Producers call it only after all outcomes and problems have been attached.
func (m Message) withHeartbeat() Message {
	m.HeartbeatMessage = renderHeartbeat(m)
	return m
}
