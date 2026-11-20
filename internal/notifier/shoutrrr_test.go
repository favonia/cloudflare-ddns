// vim: nowrap

package notifier_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDescribeShoutrrrService(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input        string
		output       string
		prepareMocks func(*mocks.MockPP)
	}{
		"ifttt": {"ifttt", "IFTTT", nil},
		"zulip": {"zulip", "Zulip Chat", nil},
		"unknown": {
			"example", "Example",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Unknown Shoutrrr service name %q; please report it at %s", "example", pp.IssueReportingURL)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			output := notifier.DescribeShoutrrrService(mockPP, tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestShoutrrrDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := notifier.NewShoutrrr(mockPP, []string{
		"generic://localhost/",
		"gotify://host:80/path/Aaa.bbb.ccc.ddd",
		"ifttt://hey/?events=1",
	})
	require.True(t, ok)

	count := 0
outer:
	for name := range m.Describe {
		count++
		switch count {
		case 1:
			require.Equal(t, "Generic", name)
		case 2:
			require.Equal(t, "Gotify", name)
			break outer
		default:
		}
	}
	require.Equal(t, 2, count)
}

func TestShoutrrrInitializationErrorDoesNotExposeCredentials(t *testing.T) {
	t.Parallel()

	const secret = "ASensitive12345"
	const validURL = "gotify://host:80/path/" + secret

	var output bytes.Buffer
	ppfmt := pp.NewDefault(&output)
	_, ok := notifier.NewShoutrrr(ppfmt, []string{validURL})
	require.True(t, ok, "the credential-bearing URL must be valid by itself: %s", output.String())

	for _, urls := range [][]string{
		{validURL, "invalid-service://host"},
		{"invalid-service://host", validURL},
	} {
		output.Reset()
		_, ok = notifier.NewShoutrrr(ppfmt, urls)
		require.False(t, ok)
		require.Contains(t, output.String(), "Failed to set up Shoutrrr notification services")
		require.NotContains(t, output.String(), secret)
	}
}

// Keep the wire format compatible with Zulip before 9.0. Shoutrrr v0.24.0
// sent type=channel, which those servers reject even for an unchanged URL.
func TestShoutrrrSendLegacyZulip(t *testing.T) {
	t.Parallel()

	notification := notifier.NewNotificationf(notifier.KindUpdate, "hello")
	var delivered atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/register":
			_, _ = io.WriteString(w, `{"result":"success","queue_id":"test-queue","max_message_length":10000,"max_topic_length":60}`)
		case "/api/v1/events":
			assert.Equal(t, http.MethodDelete, r.Method)
			_, _ = io.WriteString(w, `{"result":"success"}`)
		case "/api/v1/messages":
			if !assert.Equal(t, http.MethodPost, r.Method) || !assert.NoError(t, r.ParseForm()) {
				panic(http.ErrAbortHandler)
			}
			if r.PostForm.Get("type") != "stream" {
				http.Error(w, "unsupported message type", http.StatusBadRequest)
				return
			}
			assert.Equal(t, "updates", r.PostForm.Get("to"))
			assert.Equal(t, "DDNS", r.PostForm.Get("topic"))
			assert.Equal(t, notification.Format(), r.PostForm.Get("content"))
			delivered.Add(1)
			_, _ = io.WriteString(w, `{"result":"success","id":1}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	var output bytes.Buffer
	ppfmt := pp.NewDefault(&output)
	serviceURL := "zulip://bot%40example.com:token@" + strings.TrimPrefix(server.URL, "https://") +
		"?stream=updates&topic=DDNS&skiptlsverify=yes"
	s, ok := notifier.NewShoutrrr(ppfmt, []string{serviceURL})
	require.True(t, ok, output.String())
	require.True(t, s.Send(context.Background(), ppfmt, notification), output.String())
	require.Equal(t, int32(1), delivered.Load())
}

func TestShoutrrrSend(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		path          string
		pinged        int32
		service       func(serverURL string) string
		notification  notifier.Notification
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			"/greeting", 1,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			notifier.NewNotificationf(notifier.KindStartup, "hello"),
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(
					pp.EmojiNotify,
					"Sent %s to %s via Shoutrrr",
					"a startup notification",
					"Generic",
				)
			},
		},
		"invalid kind": {
			"/greeting", 1,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			notifier.NewNotificationf(notifier.Kind("unexpected"), "hello"),
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiImpossible,
						"Unknown notification type %q; this should not happen. Please report it at %s",
						notifier.Kind("unexpected"),
						pp.IssueReportingURL,
					),
					m.EXPECT().Infof(
						pp.EmojiNotify,
						"Sent %s to %s via Shoutrrr",
						"a notification",
						"Generic",
					),
				)
			},
		},
		"malformed url": {
			"", 0,
			func(_serverURL string) string { return "generic+https://0.0.0.0" },
			notifier.NewNotificationf(notifier.KindUpdate, "hello"),
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to send notifications via Shoutrrr: %v", gomock.Any())
			},
		},
		"empty": {
			"/greeting", 0,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			notifier.NewNotification(notifier.KindUpdate, notifier.NewMessage()),
			true,
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			var pinged atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if !assert.Equal(t, http.MethodPost, r.Method) ||
					!assert.Equal(t, tc.path, r.URL.EscapedPath()) {
					panic(http.ErrAbortHandler)
				}

				if reqBody, err := io.ReadAll(r.Body); !assert.NoError(t, err) ||
					!assert.Equal(t, tc.notification.Format(), string(reqBody)) {
					panic(http.ErrAbortHandler)
				}

				pinged.Add(1)
			}))

			t.Cleanup(server.Close)

			s, ok := notifier.NewShoutrrr(mockPP, []string{tc.service(server.URL)})
			require.True(t, ok)
			ok = s.Send(context.Background(), mockPP, tc.notification)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged.Load())
		})
	}
}

func TestShoutrrrSendMultipleServices(t *testing.T) {
	t.Parallel()

	notification := notifier.NewNotificationf(notifier.KindUpdate, "hello")
	var pinged atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if reqBody, err := io.ReadAll(r.Body); !assert.NoError(t, err) ||
			!assert.Equal(t, notification.Format(), string(reqBody)) {
			panic(http.ErrAbortHandler)
		}
		pinged.Add(1)
	}))
	t.Cleanup(server.Close)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Infof(
		pp.EmojiNotify,
		"Sent %s to %s via Shoutrrr",
		"an update notification",
		"Generic and Generic",
	)

	service := "generic+" + server.URL + "/greeting"
	s, ok := notifier.NewShoutrrr(mockPP, []string{service, service})
	require.True(t, ok)
	require.True(t, s.Send(context.Background(), mockPP, notification))
	require.Equal(t, int32(2), pinged.Load())
}
