package updater

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
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

func generateUpdateWAFListsHeartbeatMessage(s setterWAFListResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
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

	return heartbeat.Message{OK: true, Lines: successLines}
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
		HeartbeatMessage: generateUpdateWAFListsHeartbeatMessage(s),
		NotifierMessage:  generateUpdateWAFListsNotifierMessage(s),
	}
}

func generateFinalClearWAFListsHeartbeatMessage(s setterWAFListResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK:    false,
			Lines: []string{"Failed to clean WAF list(s) " + pp.Join(domains)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, "Cleaning WAF list(s) "+pp.Join(domains))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, "Cleaned WAF list(s) "+pp.Join(domains))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateFinalClearWAFListsNotifierMessage(s setterWAFListResponses) notifier.Message {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments, "Failed to properly clean WAF list(s) "+pp.EnglishJoin(domains))
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments, "Cleaning WAF list(s) ", pp.EnglishJoin(domains))
		} else {
			fragments = append(fragments, "; cleaning ", pp.EnglishJoin(domains))
		}
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments, "Cleaned WAF list(s) "+pp.EnglishJoin(domains))
		} else {
			fragments = append(fragments, "; cleaned ", pp.EnglishJoin(domains))
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
		HeartbeatMessage: generateFinalClearWAFListsHeartbeatMessage(s),
		NotifierMessage:  generateFinalClearWAFListsNotifierMessage(s),
	}
}
