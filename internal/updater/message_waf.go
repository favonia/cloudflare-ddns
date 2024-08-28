package updater

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/message"
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

func generateUpdateWAFListsMonitorMessage(s setterWAFListResponses) message.MonitorMessage {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return message.MonitorMessage{
			OK:    false,
			Lines: []string{"Failed to set list(s) " + pp.Join(domains)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, "Setting list(s) "+pp.Join(domains))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, "Set list(s) "+pp.Join(domains))
	}

	return message.MonitorMessage{OK: true, Lines: successLines}
}

func generateUpdateWAFListsNotifierMessage(s setterWAFListResponses) message.NotifierMessage {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments, "Failed to properly update WAF list(s) ", pp.EnglishJoin(domains))
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments, "Updating WAF list(s) ", pp.EnglishJoin(domains))
		} else {
			fragments = append(fragments, "; updating ", pp.EnglishJoin(domains))
		}
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments, "Updated WAF list(s) "+pp.EnglishJoin(domains))
		} else {
			fragments = append(fragments, "; updated ", pp.EnglishJoin(domains))
		}
	}

	if len(fragments) == 0 {
		return nil
	} else {
		fragments = append(fragments, ".")
		return message.NotifierMessage{strings.Join(fragments, "")}
	}
}

func generateUpdateWAFListsMessage(s setterWAFListResponses) message.Message {
	return message.Message{
		MonitorMessage:  generateUpdateWAFListsMonitorMessage(s),
		NotifierMessage: generateUpdateWAFListsNotifierMessage(s),
	}
}

func generateClearWAFListsMonitorMessage(s setterWAFListResponses) message.MonitorMessage {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return message.MonitorMessage{
			OK:    false,
			Lines: []string{"Failed to clear list(s) " + pp.Join(domains)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, "Clearing list(s) "+pp.Join(domains))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, "Cleared list(s) "+pp.Join(domains))
	}

	return message.MonitorMessage{OK: true, Lines: successLines}
}

func generateClearWAFListsNotifierMessage(s setterWAFListResponses) message.NotifierMessage {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments, "Failed to properly clear WAF list(s) "+pp.EnglishJoin(domains))
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments, "Clearing WAF list(s) ", pp.EnglishJoin(domains))
		} else {
			fragments = append(fragments, "; clearing ", pp.EnglishJoin(domains))
		}
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments, "Cleared WAF list(s) "+pp.EnglishJoin(domains))
		} else {
			fragments = append(fragments, "; cleared ", pp.EnglishJoin(domains))
		}
	}

	if len(fragments) == 0 {
		return nil
	} else {
		fragments = append(fragments, ".")
		return message.NotifierMessage{strings.Join(fragments, "")}
	}
}

func generateClearWAFListsMessage(s setterWAFListResponses) message.Message {
	return message.Message{
		MonitorMessage:  generateClearWAFListsMonitorMessage(s),
		NotifierMessage: generateClearWAFListsNotifierMessage(s),
	}
}
