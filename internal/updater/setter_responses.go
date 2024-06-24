package updater

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/response"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type SetterResponses map[setter.ResponseCode][]string

type OperationCode int

const (
	OperationUpdate OperationCode = iota
	OperationDelete
)

func (s SetterResponses) Register(code setter.ResponseCode, d domain.Domain) {
	s[code] = append(s[code], d.Describe())
}

func ListJoin(items []string) string { return strings.Join(items, ", ") }
func ListEnglishJoin(items []string) string {
	switch l := len(items); l {
	case 0:
		return "(none)"
	case 1:
		return items[0]
	case 2: //nolint:mnd
		return fmt.Sprintf("%s and %s", items[0], items[1])
	default:
		return fmt.Sprintf("%s, and %s", strings.Join(items[:l-1], ", "), items[l-1])
	}
}

func (s SetterResponses) ToResponse(op OperationCode, ipNet ipnet.Type, ip netip.Addr) response.Response {
	switch op {
	default:
		fallthrough
	case OperationUpdate:
		switch {
		case !ip.IsValid():
			return response.Response{
				Ok:               false,
				MonitorMessages:  []string{fmt.Sprintf("Failed to detect %s address", ipNet.Describe())},
				NotifierMessages: []string{fmt.Sprintf("Failed to detect the %s address.", ipNet.Describe())},
			}

		case len(s[setter.ResponseFailed]) > 0 &&
			len(s[setter.ResponseUpdated]) > 0:
			return response.Response{
				Ok: false,
				MonitorMessages: []string{fmt.Sprintf(
					"Failed to set %s (%s): %s",
					ipNet.RecordType(),
					ip.String(),
					ListJoin(s[setter.ResponseFailed]),
				)},
				NotifierMessages: []string{fmt.Sprintf(
					"Failed to finish updating %s records of %s with %s; those of %s were updated.",
					ipNet.RecordType(),
					ListEnglishJoin(s[setter.ResponseFailed]),
					ip.String(),
					ListEnglishJoin(s[setter.ResponseUpdated]),
				)},
			}

		case len(s[setter.ResponseFailed]) > 0:
			return response.Response{
				Ok: false,
				MonitorMessages: []string{fmt.Sprintf(
					"Failed to set %s (%s): %s",
					ipNet.RecordType(),
					ip.String(),
					ListJoin(s[setter.ResponseFailed]),
				)},
				NotifierMessages: []string{fmt.Sprintf(
					"Failed to finish updating %s records of %s with %s.",
					ipNet.RecordType(),
					ListEnglishJoin(s[setter.ResponseFailed]),
					ip.String(),
				)},
			}

		case len(s[setter.ResponseUpdated]) > 0:
			return response.Response{
				Ok: true,
				MonitorMessages: []string{fmt.Sprintf(
					"Set %s (%s): %s",
					ipNet.RecordType(),
					ip.String(),
					ListJoin(s[setter.ResponseUpdated]),
				)},
				NotifierMessages: []string{fmt.Sprintf(
					"Updated %s records of %s with %s.",
					ipNet.RecordType(),
					ListEnglishJoin(s[setter.ResponseUpdated]),
					ip.String(),
				)},
			}

		default:
			return response.Response{Ok: true, MonitorMessages: []string{}, NotifierMessages: []string{}}
		}

	case OperationDelete:
		switch {
		case len(s[setter.ResponseFailed]) > 0 &&
			len(s[setter.ResponseUpdated]) > 0:
			return response.Response{
				Ok: false,
				MonitorMessages: []string{fmt.Sprintf(
					"Failed to delete %s: %s",
					ipNet.RecordType(),
					ListJoin(s[setter.ResponseFailed]),
				)},
				NotifierMessages: []string{fmt.Sprintf(
					"Failed to finish deleting %s records of %s; those of %s were deleted.",
					ipNet.RecordType(),
					ListEnglishJoin(s[setter.ResponseFailed]),
					ListEnglishJoin(s[setter.ResponseUpdated]),
				)},
			}

		case len(s[setter.ResponseFailed]) > 0:
			return response.Response{
				Ok: false,
				MonitorMessages: []string{fmt.Sprintf(
					"Failed to delete %s: %s",
					ipNet.RecordType(),
					ListJoin(s[setter.ResponseFailed]),
				)},
				NotifierMessages: []string{fmt.Sprintf(
					"Failed to finish deleting %s records of %s.",
					ipNet.RecordType(),
					ListEnglishJoin(s[setter.ResponseFailed]),
				)},
			}

		case len(s[setter.ResponseUpdated]) > 0:
			return response.Response{
				Ok: true,
				MonitorMessages: []string{fmt.Sprintf(
					"Deleted %s: %s",
					ipNet.RecordType(),
					ListJoin(s[setter.ResponseUpdated]),
				)},
				NotifierMessages: []string{fmt.Sprintf(
					"Deleted %s records of %s.",
					ipNet.RecordType(),
					ListEnglishJoin(s[setter.ResponseUpdated]),
				)},
			}

		default:
			return response.Response{Ok: true, MonitorMessages: []string{}, NotifierMessages: []string{}}
		}
	}
}
