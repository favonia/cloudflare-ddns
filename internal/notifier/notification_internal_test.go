package notifier

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestNotificationDescription(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		kind Kind
		want string
	}{
		"startup":            {KindStartup, "a startup notification"},
		"startup failure":    {KindStartupFailure, "a startup failure notification"},
		"update":             {KindUpdate, "an update notification"},
		"update failure":     {KindUpdateFailure, "an update failure notification"},
		"scheduling failure": {KindSchedulingFailure, "a scheduling failure notification"},
		"cleanup":            {KindCleanup, "a cleanup notification"},
		"cleanup failure":    {KindCleanupFailure, "a cleanup failure notification"},
		"shutdown":           {KindShutdown, "a shutdown notification"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			notification := NewNotification(tc.kind, NewMessage())
			require.Equal(t, tc.want, notification.description(pp.NewSilent()))
		})
	}
}

func TestUnknownNotificationDescription(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		kind Kind
		want string
	}{
		"unexpected": {Kind("unexpected"), "\"unexpected\""},
		"empty":      {Kind(""), "\"\""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var output strings.Builder
			notification := NewNotification(tc.kind, NewMessage())
			got := notification.description(pp.NewDefault(&output))

			require.Equal(t, "a notification", got)
			require.Equal(t,
				"🤯 Unknown notification type "+tc.want+"; this should not happen. "+
					"Please report it at "+pp.IssueReportingURL+"\n",
				output.String())
		})
	}
}
