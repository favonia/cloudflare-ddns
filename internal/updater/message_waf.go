package updater

import (
	"fmt"

	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

type setterWAFListResponses map[setter.ResponseCode][]string

func emptySetterWAFListResponses() setterWAFListResponses {
	return setterWAFListResponses{}
}

func (s setterWAFListResponses) register(name string, code setter.ResponseCode) {
	s[code] = append(s[code], name)
}

//nolint:dupl
func generateUpdateWAFListMessage(s setterWAFListResponses) message.Message {
	switch {
	case len(s[setter.ResponseFailed]) > 0 &&
		len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{
				"Failed to set list(s): " + pp.Join(s[setter.ResponseFailed]),
			},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly update WAF list(s) %s; %s were updated.",
				pp.EnglishJoin(s[setter.ResponseFailed]),
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	case len(s[setter.ResponseFailed]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{
				"Failed to set list(s): " + pp.Join(s[setter.ResponseFailed]),
			},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly update WAF list(s) %s.",
				pp.EnglishJoin(s[setter.ResponseFailed]),
			)},
		}

	case len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: true,
			MonitorMessages: []string{
				"Set list(s): " + pp.Join(s[setter.ResponseUpdated]),
			},
			NotifierMessages: []string{fmt.Sprintf(
				"Updated WAF list(s) %s.",
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	default:
		return message.Message{OK: true, MonitorMessages: []string{}, NotifierMessages: []string{}}
	}
}

//nolint:dupl
func generateDeleteWAFListMessage(s setterWAFListResponses) message.Message {
	switch {
	case len(s[setter.ResponseFailed]) > 0 &&
		len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{
				"Failed to delete list(s): " + pp.Join(s[setter.ResponseFailed]),
			},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly delete WAF list(s) %s; %s were deleted.",
				pp.EnglishJoin(s[setter.ResponseFailed]),
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	case len(s[setter.ResponseFailed]) > 0:
		return message.Message{
			OK: false,
			MonitorMessages: []string{
				"Failed to delete list(s): " + pp.Join(s[setter.ResponseFailed]),
			},
			NotifierMessages: []string{fmt.Sprintf(
				"Failed to properly delete WAF list(s) %s.",
				pp.EnglishJoin(s[setter.ResponseFailed]),
			)},
		}

	case len(s[setter.ResponseUpdated]) > 0:
		return message.Message{
			OK: true,
			MonitorMessages: []string{
				"Deleted list(s): " + pp.Join(s[setter.ResponseUpdated]),
			},
			NotifierMessages: []string{fmt.Sprintf(
				"Deleted WAF list(s) %s.",
				pp.EnglishJoin(s[setter.ResponseUpdated]),
			)},
		}

	default:
		return message.Message{OK: true, MonitorMessages: []string{}, NotifierMessages: []string{}}
	}
}
