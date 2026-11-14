package updater

import (
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// renderHeartbeat projects work evidence into the existing terse heartbeat format.
// Family order, target grouping and failure-only selection are presentation policy;
// they never determine whether work proceeds or whether the round failed.
func renderHeartbeat(message Message) heartbeat.Message {
	var parts []heartbeat.Message
	for _, family := range []ipnet.Family{ipnet.IP4, ipnet.IP6} {
		for _, problem := range message.problems {
			if problem.family == family {
				parts = append(parts, renderProblemHeartbeat(problem))
			}
		}
		type group struct {
			cleanup bool
			target  []netip.Addr
			results setterResponses
		}
		var groups []group
		for _, outcome := range message.dns {
			if outcome.scope.family != family {
				continue
			}
			index := slices.IndexFunc(groups, func(g group) bool {
				return g.cleanup == outcome.cleanup && slices.Equal(g.target, outcome.target)
			})
			if index < 0 {
				groups = append(groups, group{cleanup: outcome.cleanup, target: outcome.target, results: emptySetterResponses()})
				index = len(groups) - 1
			}
			groups[index].results.register(outcome.scope.domain, outcome.result.Code)
		}
		for _, group := range groups {
			switch {
			case group.cleanup:
				parts = append(parts, generateFinalDeleteHeartbeatMessage(family, group.results))
			case isTargetSetEmpty(group.target):
				parts = append(parts, generateClearHeartbeatMessage(family, group.results))
			default:
				parts = append(parts, generateUpdateHeartbeatMessage(family, group.target, group.results))
			}
		}
	}
	type wafGroup struct {
		cleanup bool
		results setterWAFListResponses
	}
	var groups []wafGroup
	for _, outcome := range message.waf {
		index := slices.IndexFunc(groups, func(g wafGroup) bool { return g.cleanup == outcome.cleanup })
		if index < 0 {
			groups = append(groups, wafGroup{cleanup: outcome.cleanup, results: emptySetterWAFListResponses()})
			index = len(groups) - 1
		}
		groups[index].results.register(outcome.list.Describe(), outcome.result.Code)
	}
	for _, group := range groups {
		if group.cleanup {
			parts = append(parts, generateFinalClearWAFListsHeartbeatMessage(group.results))
		} else {
			parts = append(parts, generateUpdateWAFListsHeartbeatMessage(group.results))
		}
	}
	result := heartbeat.MergeMessages(parts...)
	result.OK = !message.Failed()
	return result
}
