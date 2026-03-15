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

func generateDetectMessage(ipFamily ipnet.Family, ok bool) Message {
	switch {
	default:
		return NewMessage()
	case !ok:
		return Message{
			HeartbeatMessage: heartbeat.Message{
				OK:    false,
				Lines: []string{fmt.Sprintf("Failed to detect any %s addresses", ipFamily.Describe())},
			},
			NotifierMessage: notifier.Message{
				fmt.Sprintf("Failed to detect any %s addresses.", ipFamily.Describe()),
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

func generateUpdateHeartbeatMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) heartbeat.Message {
	ipDescription := describeIPs(ips)

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm update of %s (%s) for %s",
				ipFamily.RecordType(), ipDescription, pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Setting %s (%s) of %s",
			ipFamily.RecordType(), ipDescription, pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Set %s (%s) of %s",
			ipFamily.RecordType(), ipDescription, pp.Join(domains),
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateUpdateNotifierMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) notifier.Message {
	ipDescription := describeIPsInEnglish(ips)
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments,
			"Could not confirm update of ", ipFamily.RecordType(),
			" records of ", pp.EnglishJoin(domains), " with ", ipDescription,
		)
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Updating ", ipFamily.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ipDescription,
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
				"Updated ", ipFamily.RecordType(), " records of ", pp.EnglishJoin(domains), " with ", ipDescription,
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

func generateUpdateMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) Message {
	return Message{
		HeartbeatMessage: generateUpdateHeartbeatMessage(ipFamily, ips, s),
		NotifierMessage:  generateUpdateNotifierMessage(ipFamily, ips, s),
	}
}

func generateFinalDeleteHeartbeatMessage(ipFamily ipnet.Family, s setterResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm deletion of %s of %s",
				ipFamily.RecordType(), pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleting %s of %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleted %s of %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateFinalDeleteNotifierMessage(ipFamily ipnet.Family, s setterResponses) notifier.Message {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments,
			"Could not confirm deletion of ", ipFamily.RecordType(), " records of ", pp.EnglishJoin(domains),
		)
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		if len(fragments) == 0 {
			fragments = append(fragments,
				"Deleting ", ipFamily.RecordType(), " records of ", pp.EnglishJoin(domains),
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
				"Deleted ", ipFamily.RecordType(), " records of ", pp.EnglishJoin(domains),
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

func generateFinalDeleteMessage(ipFamily ipnet.Family, s setterResponses) Message {
	return Message{
		HeartbeatMessage: generateFinalDeleteHeartbeatMessage(ipFamily, s),
		NotifierMessage:  generateFinalDeleteNotifierMessage(ipFamily, s),
	}
}
