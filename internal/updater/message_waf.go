package updater

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
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

func generateUpdateWAFListsMonitorMessage(s setterWAFListResponses) monitor.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return monitor.Message{
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

	return monitor.Message{OK: true, Lines: successLines}
}

func generateUpdateWAFListsNotifierMessage(s setterWAFListResponses) notifier.Message {
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
		return notifier.Message{strings.Join(fragments, "")}
	}
}

func generateUpdateWAFListsMessage(s setterWAFListResponses) Message {
	return Message{
		MonitorMessage:  generateUpdateWAFListsMonitorMessage(s),
		NotifierMessage: generateUpdateWAFListsNotifierMessage(s),
	}
}

func generateFinalClearWAFListsMonitorMessage(s setterWAFListResponses) monitor.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return monitor.Message{
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

	return monitor.Message{OK: true, Lines: successLines}
}

func generateFinalClearWAFListsNotifierMessage(s setterWAFListResponses) notifier.Message {
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
		return notifier.Message{strings.Join(fragments, "")}
	}
}

func generateFinalClearWAFListsMessage(s setterWAFListResponses) Message {
	return Message{
		MonitorMessage:  generateFinalClearWAFListsMonitorMessage(s),
		NotifierMessage: generateFinalClearWAFListsNotifierMessage(s),
	}
}
