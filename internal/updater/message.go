package updater

import (
	"fmt"
	"net/netip"

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
		return message.NewEmpty()
	case !ok:
		return message.Message{
			OK:               false,
			MonitorMessages:  []string{fmt.Sprintf("Failed to detect %s address", ipNet.Describe())},
			NotifierMessages: []string{fmt.Sprintf("Failed to detect the %s address.", ipNet.Describe())},
		}
	}
}

func generateUpdateMessage(ipNet ipnet.Type, ip netip.Addr, s setterResponses) message.Message {
	switch {
	case len(s[setter.ResponseFailed]) > 0 &&
		len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{fmt.Sprintf(
				"Failed to set %s (%s): %s",
				ipNet.RecordType(),
				ip.String(),
				pp.Join(s[setter.ResponseFailed]),
			)},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly update %s records of %s with %s; those of %s were updated.",
				ipNet.RecordType(),
				pp.EnglishJoin(s[setter.ResponseFailed]),
				ip.String(),
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	case len(s[setter.ResponseFailed]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{fmt.Sprintf(
				"Failed to set %s (%s): %s",
				ipNet.RecordType(),
				ip.String(),
				pp.Join(s[setter.ResponseFailed]),
			)},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly update %s records of %s with %s.",
				ipNet.RecordType(),
				pp.EnglishJoin(s[setter.ResponseFailed]),
				ip.String(),
			)},
		}

	case len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: true,
			MonitorMessages: []string{fmt.Sprintf(
				"Set %s (%s): %s",
				ipNet.RecordType(),
				ip.String(),
				pp.Join(s[setter.ResponseUpdated]),
			)},
			NotifierMessages: []string{fmt.Sprintf(
				"Updated %s records of %s with %s.",
				ipNet.RecordType(),
				pp.EnglishJoin(s[setter.ResponseUpdated]),
				ip.String(),
			)},
		}

	default:
		return message.Message{OK: true, MonitorMessages: []string{}, NotifierMessages: []string{}}
	}
}

func generateDeleteMessage(ipNet ipnet.Type, s setterResponses) message.Message {
	switch {
	case len(s[setter.ResponseFailed]) > 0 &&
		len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{fmt.Sprintf(
				"Failed to delete %s: %s",
				ipNet.RecordType(),
				pp.Join(s[setter.ResponseFailed]),
			)},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly delete %s records of %s; those of %s were deleted.",
				ipNet.RecordType(),
				pp.EnglishJoin(s[setter.ResponseFailed]),
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	case len(s[setter.ResponseFailed]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{fmt.Sprintf(
				"Failed to delete %s: %s",
				ipNet.RecordType(),
				pp.Join(s[setter.ResponseFailed]),
			)},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly delete %s records of %s.",
				ipNet.RecordType(),
				pp.EnglishJoin(s[setter.ResponseFailed]),
			)},
		}

	case len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: true,
			MonitorMessages: []string{fmt.Sprintf(
				"Deleted %s: %s",
				ipNet.RecordType(),
				pp.Join(s[setter.ResponseUpdated]),
			)},
			NotifierMessages: []string{fmt.Sprintf(
				"Deleted %s records of %s.",
				ipNet.RecordType(),
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	default:
		return message.Message{OK: true, MonitorMessages: []string{}, NotifierMessages: []string{}}
	}
}
