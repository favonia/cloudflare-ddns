package updater

import (
	"fmt"
	"strings"
)

// These neutral semantic renderers intentionally defer production wording until
// the integrated operator-message review. They retain every grouping distinction.
func renderDNS(row dnsRow) string {
	completion := make([]string, 0, len(row.completion))
	for _, fact := range row.completion {
		completion = append(completion, fmt.Sprintf("status=%d removal=%t address=%s membership=%d",
			fact.status, fact.removal, fact.address, fact.membership))
	}
	return fmt.Sprintf(
		"dns domains=%v family=%v cleanup=%t outcome=%d before-known=%t before=%v target=%v completion=[%s] unexplained=%t",
		row.domains, row.family, row.cleanup, row.code, row.beforeKnown, row.before, row.target,
		strings.Join(completion, "; "), row.unexplained,
	)
}

func renderWAF(o wafOutcome) string {
	return fmt.Sprintf("waf list=%s cleanup=%t outcome=%d", o.list.Describe(), o.cleanup, o.code)
}

func renderProblem(p operationProblem) string {
	return fmt.Sprintf("problem kind=%d family=%v domains=%v", p.kind, p.family, p.domains)
}

func renderLifecycle(lifecycle LifecycleContext) string {
	return fmt.Sprintf("lifecycle kind=%d schedule=%s", lifecycle.Kind, lifecycle.Schedule)
}
