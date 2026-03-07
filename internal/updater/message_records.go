package updater

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
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

func generateDetectMessage(ipNet ipnet.Type, ok bool) Message {
	switch {
	default:
		return NewMessage()
	case !ok:
		return Message{
			HeartbeatMessage: heartbeat.Message{
				OK:    false,
				Lines: []string{fmt.Sprintf("Failed to detect any %s addresses", ipNet.Describe())},
			},
			NotifierMessage: notifier.Message{
				fmt.Sprintf("Failed to detect any %s addresses.", ipNet.Describe()),
			},
		}
	}
}

func describeIPs(ips []netip.Addr) string {
	return pp.JoinMap(netip.Addr.String, ips)
}

func describeIPsInEnglish(ips []netip.Addr) string {
	return pp.EnglishJoinMap(netip.Addr.String, ips)
}

func generateUpdateHeartbeatMessage(ipNet ipnet.Type, ips []netip.Addr, s setterResponses) heartbeat.Message {
	ipDescription := describeIPs(ips)

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm update of %s (%s) for %s",
				ipNet.RecordType(), ipDescription, pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Setting %s (%s) of %s",
			ipNet.RecordType(), ipDescription, pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Set %s (%s) of %s",
			ipNet.RecordType(), ipDescription, pp.Join(domains),
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateUpdateNotifierMessage(ipNet ipnet.Type, ips []netip.Addr, s setterResponses) notifier.Message {
	ipDescription := describeIPsInEnglish(ips)
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments,
			"Could not confirm update of ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ipDescription,
		)
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Updating ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ipDescription,
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
				"Updated ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ipDescription,
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
		return notifier.Message{strings.Join(fragments, "")}
	}
}

func generateUpdateMessage(ipNet ipnet.Type, ips []netip.Addr, s setterResponses) Message {
	return Message{
		HeartbeatMessage: generateUpdateHeartbeatMessage(ipNet, ips, s),
		NotifierMessage:  generateUpdateNotifierMessage(ipNet, ips, s),
	}
}

func generateFinalDeleteHeartbeatMessage(ipNet ipnet.Type, s setterResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm deletion of %s of %s",
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

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateFinalDeleteNotifierMessage(ipNet ipnet.Type, s setterResponses) notifier.Message {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments,
			"Could not confirm deletion of ", ipNet.RecordType(), " records of ", pp.EnglishJoin(domains),
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
		return notifier.Message{strings.Join(fragments, "")}
	}
}

func generateFinalDeleteMessage(ipNet ipnet.Type, s setterResponses) Message {
	return Message{
		HeartbeatMessage: generateFinalDeleteHeartbeatMessage(ipNet, s),
		NotifierMessage:  generateFinalDeleteNotifierMessage(ipNet, s),
	}
}
