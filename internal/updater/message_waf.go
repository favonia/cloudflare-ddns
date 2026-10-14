package updater

import (
	"fmt"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type setterWAFListResponses map[setter.ResponseCode][]string

func emptySetterWAFListResponses() setterWAFListResponses {
	return setterWAFListResponses{}
}

func (s setterWAFListResponses) register(name string, code setter.ResponseCode) {
	s[code] = append(s[code], name)
}

// Heartbeat success lines are intentionally terse status labels. Failure lines
// can be longer because they are the ones users need to inspect. See
// docs/design/guides/operator-messages.markdown.
func generateUpdateWAFListsHeartbeatMessage(s setterWAFListResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK:    false,
			Lines: []string{fmt.Sprintf("Could not confirm update of WAF list(s) %s", pp.Join(domains))},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Updating WAF list(s) %s", pp.Join(domains)))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Updated WAF list(s) %s", pp.Join(domains)))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateUpdateWAFListsMessage(s setterWAFListResponses) Message {
	return Message{
		HeartbeatMessage: generateUpdateWAFListsHeartbeatMessage(s),
		dns:              nil,
		waf:              nil,
		problems:         nil,
		cleanup:          false,
	}
}

func generateFinalClearWAFListsHeartbeatMessage(s setterWAFListResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK:    false,
			Lines: []string{fmt.Sprintf("Could not confirm cleanup of WAF list(s) %s", pp.Join(domains))},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Cleaning WAF list(s) %s", pp.Join(domains)))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Cleaned WAF list(s) %s", pp.Join(domains)))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateFinalClearWAFListsMessage(s setterWAFListResponses) Message {
	return Message{
		HeartbeatMessage: generateFinalClearWAFListsHeartbeatMessage(s),
		dns:              nil,
		waf:              nil,
		problems:         nil,
		cleanup:          true,
	}
}
