package updater

import (
	"fmt"
	"net/netip"

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

// Heartbeat success messages stay compact because heartbeat services mainly
// surface short status text. Failures can spend more words because they are the
// messages users need to inspect. Notifier messages are more prose-like for
// Shoutrrr and similar channels, so they use English joins instead.
func describeIPsInEnglish(ips []netip.Addr) string {
	return pp.EnglishJoinMapOrEmptyLabel(netip.Addr.String, ips, "(none)")
}

func describeDomainsInEnglish(domains []string) string {
	return pp.EnglishJoinOrEmptyLabel(domains, "(none)")
}

// isTargetSetEmpty reports whether the target IP set is empty.
// An empty target set signals record clearing (as produced by static.empty).
func isTargetSetEmpty(ips []netip.Addr) bool {
	return len(ips) == 0
}

func generateClearHeartbeatMessage(ipFamily ipnet.Family, s setterResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm that %s records for %s were cleared",
				ipFamily.RecordType(), pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Clearing %s records for %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Cleared %s records for %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateClearNotifierMessage(ipFamily ipnet.Family, s setterResponses) notifier.Message {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments, fmt.Sprintf(
			"Could not confirm that %s records for %s were cleared",
			ipFamily.RecordType(), describeDomainsInEnglish(domains)))
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		fragments = appendNotifierFragmentf(
			fragments,
			"Clearing %s records for %s",
			"; clearing %s records for %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains),
		)
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		fragments = appendNotifierFragmentf(
			fragments,
			"Cleared %s records for %s",
			"; cleared %s records for %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains),
		)
	}

	return finishNotifierMessage(fragments)
}

func generateUpdateHeartbeatMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) heartbeat.Message {
	ipDescription := describeIPs(ips)

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm that %s records for %s were updated to %s",
				ipFamily.RecordType(), pp.Join(domains), ipDescription,
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Setting %s records for %s to %s",
			ipFamily.RecordType(), pp.Join(domains), ipDescription,
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Set %s records for %s to %s",
			ipFamily.RecordType(), pp.Join(domains), ipDescription,
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateUpdateNotifierMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) notifier.Message {
	ipDescription := describeIPsInEnglish(ips)
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments, fmt.Sprintf(
			"Could not confirm that %s records for %s were updated to %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains), ipDescription))
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		fragments = appendNotifierFragmentf(
			fragments,
			"Updating %s records for %s to %s",
			"; updating %s records for %s to %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains), ipDescription,
		)
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		fragments = appendNotifierFragmentf(
			fragments,
			"Updated %s records for %s to %s",
			"; updated %s records for %s to %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains), ipDescription,
		)
	}

	return finishNotifierMessage(fragments)
}

func generateClearOrUpdateMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) Message {
	if isTargetSetEmpty(ips) {
		return Message{
			HeartbeatMessage: generateClearHeartbeatMessage(ipFamily, s),
			NotifierMessage:  generateClearNotifierMessage(ipFamily, s),
		}
	} else {
		return Message{
			HeartbeatMessage: generateUpdateHeartbeatMessage(ipFamily, ips, s),
			NotifierMessage:  generateUpdateNotifierMessage(ipFamily, ips, s),
		}
	}
}

func generateFinalDeleteHeartbeatMessage(ipFamily ipnet.Family, s setterResponses) heartbeat.Message {
	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		return heartbeat.Message{
			OK: false,
			Lines: []string{fmt.Sprintf(
				"Could not confirm that %s records for %s were deleted",
				ipFamily.RecordType(), pp.Join(domains),
			)},
		}
	}

	var successLines []string

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleting %s records for %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleted %s records for %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}

func generateFinalDeleteNotifierMessage(ipFamily ipnet.Family, s setterResponses) notifier.Message {
	var fragments []string

	if domains := s[setter.ResponseFailed]; len(domains) > 0 {
		fragments = append(fragments, fmt.Sprintf(
			"Could not confirm that %s records for %s were deleted",
			ipFamily.RecordType(), describeDomainsInEnglish(domains)))
	}

	if domains := s[setter.ResponseUpdating]; len(domains) > 0 {
		fragments = appendNotifierFragmentf(
			fragments,
			"Deleting %s records for %s",
			"; deleting %s records for %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains),
		)
	}

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		fragments = appendNotifierFragmentf(
			fragments,
			"Deleted %s records for %s",
			"; deleted %s records for %s",
			ipFamily.RecordType(), describeDomainsInEnglish(domains),
		)
	}

	return finishNotifierMessage(fragments)
}

func generateFinalDeleteMessage(ipFamily ipnet.Family, s setterResponses) Message {
	return Message{
		HeartbeatMessage: generateFinalDeleteHeartbeatMessage(ipFamily, s),
		NotifierMessage:  generateFinalDeleteNotifierMessage(ipFamily, s),
	}
}
