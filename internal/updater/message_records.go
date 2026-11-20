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

// setterResponses collects synchronous DNS outcomes: failed, updated, or no-op.
// ResponseUpdating belongs to asynchronous WAF operations, not DNS reconciliation.
type setterResponses map[setter.ResponseCode][]string

func emptySetterResponses() setterResponses {
	return setterResponses{}
}

func (s setterResponses) register(d domain.Domain, code setter.ResponseCode) {
	s[code] = append(s[code], d.Describe())
}

func generateDetectMessage(ipFamily ipnet.Family, ok bool) Message {
	if ok {
		return newMessage()
	}
	return problemMessage(operationProblem{kind: problemDetection, family: ipFamily, domains: nil})
}

func generateFilterAbortDetectMessage(ipFamily ipnet.Family) Message {
	return problemMessage(operationProblem{kind: problemFiltering, family: ipFamily, domains: nil})
}

func generateIP6DerivationFailureMessage(domains []domain.Domain) Message {
	return problemMessage(operationProblem{kind: problemDerivation, family: ipnet.IP6, domains: slices.Clone(domains)})
}

func generateMissingTargetSetsMessage(ipFamily ipnet.Family, domains []domain.Domain) Message {
	return problemMessage(operationProblem{kind: problemMissingTarget, family: ipFamily, domains: slices.Clone(domains)})
}

func problemMessage(problem operationProblem) Message {
	message := newMessage()
	message.problems = []operationProblem{problem}
	return message.withHeartbeat()
}

func renderProblemHeartbeat(problem operationProblem) heartbeat.Message {
	var line string
	switch problem.kind {
	case problemDetection:
		line = fmt.Sprintf("Failed to detect any %s addresses", problem.family.Describe())
	case problemFiltering:
		line = fmt.Sprintf("No detected %s addresses remain after filtering", problem.family.Describe())
	case problemDerivation:
		line = "No AAAA records were changed because a hostid6 setting is incompatible with the detected IPv6 prefixes"
	case problemMissingTarget:
		line = fmt.Sprintf("Could not update %s records for %s", problem.family.RecordType(),
			pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, problem.domains, "(none)"))
	}
	return heartbeat.Message{OK: false, Lines: []string{line}}
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

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Set %s records for %s to %s",
			ipFamily.RecordType(), pp.Join(domains), ipDescription,
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
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

	if domains := s[setter.ResponseUpdated]; len(domains) > 0 {
		successLines = append(successLines, fmt.Sprintf(
			"Deleted %s records for %s",
			ipFamily.RecordType(), pp.Join(domains),
		))
	}

	return heartbeat.Message{OK: true, Lines: successLines}
}
