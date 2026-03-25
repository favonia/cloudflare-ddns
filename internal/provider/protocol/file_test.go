// vim: nowrap

package protocol_test

import (
	"context"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func useMemFS(t *testing.T, memfs fstest.MapFS) {
	t.Helper()
	file.FS = memfs
	t.Cleanup(func() { file.FS = os.DirFS("/") })
}

func memFile(data string) *fstest.MapFile {
	return &fstest.MapFile{
		Data:    []byte(data),
		Mode:    0o644,
		ModTime: time.Unix(1234, 5678),
		Sys:     nil,
	}
}

//nolint:paralleltest // changing global var file.FS
func TestFileName(t *testing.T) {
	p := protocol.NewFile("file:/etc/ips.txt", "/etc/ips.txt")
	require.Equal(t, "file:/etc/ips.txt", p.Name())
}

//nolint:paralleltest // changing global var file.FS
func TestFileIsExplicitEmpty(t *testing.T) {
	p := protocol.NewFile("file:/etc/ips.txt", "/etc/ips.txt")
	require.False(t, p.IsExplicitEmpty())
}

//nolint:paralleltest // changing global var file.FS
func TestFileGetRawData(t *testing.T) {
	for name, tc := range map[string]struct {
		content       string
		ipFamily      ipnet.Family
		ok            bool
		expected      []ipnet.RawEntry
		prepareMockPP func(*mocks.MockPP)
	}{
		"single-ip4": {
			"1.1.1.1\n",
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{mustRawEntry("1.1.1.1/32")},
			nil,
		},
		"single-ip6": {
			"2001:db8::1\n",
			ipnet.IP6,
			true,
			[]ipnet.RawEntry{mustRawEntry("2001:db8::1/64")},
			nil,
		},
		"multiple-ips": {
			"2.2.2.2\n1.1.1.1\n",
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{mustRawEntry("1.1.1.1/32"), mustRawEntry("2.2.2.2/32")},
			nil,
		},
		"duplicates": {
			"1.1.1.1\n2.2.2.2\n1.1.1.1\n",
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{mustRawEntry("1.1.1.1/32"), mustRawEntry("2.2.2.2/32")},
			nil,
		},
		"comments-and-blanks": {
			"# header comment\n\n1.1.1.1 # home\n\n# footer\n",
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{mustRawEntry("1.1.1.1/32")},
			nil,
		},
		"empty-file": {
			"",
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{},
			nil,
		},
		"comment-only": {
			"# just a comment\n",
			ipnet.IP4,
			true,
			[]ipnet.RawEntry{},
			nil,
		},
		"malformed-entry": {
			"not-an-ip\n",
			ipnet.IP4,
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Failed to parse line %d (%q) of %s as an IP address or an IP address in CIDR notation", 1, "not-an-ip", "/ips.txt")
			},
		},
		"zone-identifier": {
			"1::1%eth0\n",
			ipnet.IP6,
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Failed to parse line %d (%q) of %s as an IP address or an IP address in CIDR notation",
					1, "1::1%eth0", "/ips.txt")
			},
		},
		"is4in6": {
			"::ffff:1.1.1.1\n",
			ipnet.IP6,
			false, nil,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError,
						"Line %d (%q) of %s %s",
						1, "::ffff:1.1.1.1", "/ips.txt", "is an IPv4-mapped IPv6 address"),
					m.EXPECT().InfoOncef(pp.MessageIP4MappedIP6Address, pp.EmojiHint,
						"An IPv4-mapped IPv6 address is an IPv4 address in disguise. It cannot be used for routing IPv6 traffic. If you need to use it for DNS, please open an issue at %s",
						pp.IssueReportingURL),
				)
			},
		},
		"family-mismatch": {
			"2001:db8::1\n",
			ipnet.IP4,
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Line %d (%q) of %s %s",
					1, "2001:db8::1", "/ips.txt", "is not a valid IPv4 address")
			},
		},
		"loopback": {
			"127.0.0.1\n",
			ipnet.IP4,
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Line %d (%q) of %s %s",
					1, "127.0.0.1", "/ips.txt", "is a loopback address")
			},
		},
		"unspecified": {
			"0.0.0.0\n",
			ipnet.IP4,
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Line %d (%q) of %s %s",
					1, "0.0.0.0", "/ips.txt", "is an unspecified address")
			},
		},
		"link-local": {
			"169.254.1.1\n",
			ipnet.IP4,
			false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					"Line %d (%q) of %s %s",
					1, "169.254.1.1", "/ips.txt", "is a link-local address")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			useMemFS(t, fstest.MapFS{
				"ips.txt": memFile(tc.content),
			})

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			p := protocol.NewFile("file:/ips.txt", "/ips.txt")
			result := p.GetRawData(context.Background(), mockPP, tc.ipFamily, testDefaultPrefixLen(tc.ipFamily))
			require.Equal(t, tc.ok, result.Available)
			if tc.ok {
				require.Equal(t, tc.expected, result.RawEntries)
			} else {
				require.Empty(t, result.RawEntries)
			}
		})
	}
}

//nolint:paralleltest // changing global var file.FS
func TestFileGetRawDataMissingFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	useMemFS(t, fstest.MapFS{})

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to read %q: %v", "/missing.txt", gomock.Any())

	p := protocol.NewFile("file:/missing.txt", "/missing.txt")
	result := p.GetRawData(context.Background(), mockPP, ipnet.IP4, 32)
	require.False(t, result.Available)
	require.Empty(t, result.RawEntries)
}
