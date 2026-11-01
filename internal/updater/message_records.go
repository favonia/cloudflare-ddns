package updater

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
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
		return newMessage()
	case !ok:
		return Message{
			HeartbeatMessage: heartbeat.Message{
				OK:    false,
				Lines: []string{fmt.Sprintf("Failed to detect any %s addresses", ipFamily.Describe())},
			},
			dns:      nil,
			waf:      nil,
			problems: []operationProblem{{kind: problemDetection, family: ipFamily, domains: nil}},
			cleanup:  false,
		}
	}
}

func generateFilterAbortDetectMessage(ipFamily ipnet.Family) Message {
	summary := fmt.Sprintf("No detected %s addresses remain after filtering", ipFamily.Describe())
	return Message{
		HeartbeatMessage: heartbeat.Message{
			OK:    false,
			Lines: []string{summary},
		},
		dns:      nil,
		waf:      nil,
		problems: []operationProblem{{kind: problemFiltering, family: ipFamily, domains: nil}},
		cleanup:  false,
	}
}

func generateIP6DerivationFailureMessage(domains []domain.Domain) Message {
	message := "No AAAA records were changed because a hostid6 setting is incompatible with the detected IPv6 prefixes"
	return Message{
		HeartbeatMessage: heartbeat.Message{
			OK:    false,
			Lines: []string{message},
		},
		dns:      nil,
		waf:      nil,
		problems: []operationProblem{{kind: problemDerivation, family: ipnet.IP6, domains: slices.Clone(domains)}},
		cleanup:  false,
	}
}

func generateMissingTargetSetsMessage(ipFamily ipnet.Family, domains []domain.Domain) Message {
	domainDescription := pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, domains, "(none)")
	summary := fmt.Sprintf("Could not update %s records for %s", ipFamily.RecordType(), domainDescription)
	return Message{
		HeartbeatMessage: heartbeat.Message{OK: false, Lines: []string{summary}},
		dns:              nil,
		waf:              nil,
		problems:         []operationProblem{{kind: problemMissingTarget, family: ipFamily, domains: slices.Clone(domains)}},
		cleanup:          false,
	}
}

func describeIPs(ips []netip.Addr) string {
	return pp.JoinMap(netip.Addr.String, ips)
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

func generateClearOrUpdateMessage(ipFamily ipnet.Family, ips []netip.Addr, s setterResponses) Message {
	if isTargetSetEmpty(ips) {
		return Message{
			HeartbeatMessage: generateClearHeartbeatMessage(ipFamily, s),
			dns:              nil,
			waf:              nil,
			problems:         nil,
			cleanup:          false,
		}
	} else {
		return Message{
			HeartbeatMessage: generateUpdateHeartbeatMessage(ipFamily, ips, s),
			dns:              nil,
			waf:              nil,
			problems:         nil,
			cleanup:          false,
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

func generateFinalDeleteMessage(ipFamily ipnet.Family, s setterResponses) Message {
	return Message{
		HeartbeatMessage: generateFinalDeleteHeartbeatMessage(ipFamily, s),
		dns:              nil,
		waf:              nil,
		problems:         nil,
		cleanup:          true,
	}
}
