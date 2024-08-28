package updater

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type setterResponses map[setter.ResponseCode][]string

func emptySetterResponses() setterResponses {
	return setterResponses{}
}

func (s setterResponses) register(d domain.Domain, code setter.ResponseCode) {
	s[code] = append(s[code], d.Describe())
}

func generateDetectMessage(ipNet ipnet.Type, ok bool) message.Message {
	switch {
	default:
		return message.New()
	case !ok:
		return message.Message{
			MonitorMessage: message.MonitorMessage{
				OK:    false,
				Lines: []string{fmt.Sprintf("Failed to detect %s address", ipNet.Describe())},
			},
			NotifierMessage: message.NotifierMessage{
				fmt.Sprintf("Failed to detect the %s address.", ipNet.Describe()),
			},
		}
	}
}

func generateUpdateMonitorMessage(ipNet ipnet.Type, ip netip.Addr, s setterResponses) message.MonitorMessage {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return message.MonitorMessage{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Failed to set %s (%s) of %s",
				ipNet.RecordType(), ip.String(), pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Setting %s (%s) of %s",
			ipNet.RecordType(), ip.String(), pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Set %s (%s) of %s",
			ipNet.RecordType(), ip.String(), pp.Join(domains),
		))
	}

	return message.MonitorMessage{OK: true, Lines: successLines}
}

func generateUpdateNotifierMessage(ipNet ipnet.Type, ip netip.Addr, s setterResponses) message.NotifierMessage {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments,
			"Failed to properly update ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ip.String(),
		)
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Updating ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ip.String(),
			)
		} else {
			fragments = append(fragments,
				"; updating those of ", pp.EnglishJoin(domains),
			)
		}
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Updated ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ip.String(),
			)
		} else {
			fragments = append(fragments,
				"; updated those of ", pp.EnglishJoin(domains),
			)
		}
	}

	if len(fragments) == 0 {
		return nil
	} else {
		fragments = append(fragments, ".")
		return message.NotifierMessage{strings.Join(fragments, "")}
	}
}

func generateUpdateMessage(ipNet ipnet.Type, ip netip.Addr, s setterResponses) message.Message {
	return message.Message{
		MonitorMessage:  generateUpdateMonitorMessage(ipNet, ip, s),
		NotifierMessage: generateUpdateNotifierMessage(ipNet, ip, s),
	}
}

func generateDeleteMonitorMessage(ipNet ipnet.Type, s setterResponses) message.MonitorMessage {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return message.MonitorMessage{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Failed to delete %s of %s",
				ipNet.RecordType(), pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleting %s of %s",
			ipNet.RecordType(), pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleted %s of %s",
			ipNet.RecordType(), pp.Join(domains),
		))
	}

	return message.MonitorMessage{OK: true, Lines: successLines}
}

func generateDeleteNotifierMessage(ipNet ipnet.Type, s setterResponses) message.NotifierMessage {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments,
			"Failed to properly delete ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains),
		)
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Deleting ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains),
			)
		} else {
			fragments = append(fragments,
				"; deleting those of ", pp.EnglishJoin(domains),
			)
		}
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Deleted ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains),
			)
		} else {
			fragments = append(fragments,
				"; deleted those of ", pp.EnglishJoin(domains),
			)
		}
	}

	if len(fragments) == 0 {
		return nil
	} else {
		fragments = append(fragments, ".")
		return message.NotifierMessage{strings.Join(fragments, "")}
	}
}

func generateDeleteMessage(ipNet ipnet.Type, s setterResponses) message.Message {
	return message.Message{
		MonitorMessage:  generateDeleteMonitorMessage(ipNet, s),
		NotifierMessage: generateDeleteNotifierMessage(ipNet, s),
	}
}
