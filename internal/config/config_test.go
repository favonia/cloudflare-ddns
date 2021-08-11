package config_test

import (
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDefaultConfigNotNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, config.Default())
}

//nolint:paralleltest // environment vars are global
func TestReadAuthToken(t *testing.T) {
	unset("CF_API_TOKEN")
	unset("CF_API_TOKEN_FILE")
	unset("CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token     string
		account   string
		ok        bool
		ppRecords []pp.Record
	}{
		"full":      {"123456789", "secret account", true, nil},
		"noaccount": {"123456789", "", true, nil},
		"notoken": {
			"", "account", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Needs either CF_API_TOKEN or CF_API_TOKEN_FILE`),
			},
		},
		"copycat": {
			"YOUR-CLOUDFLARE-API-TOKEN", "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `You need to provide a real API token as CF_API_TOKEN`),
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("CF_API_TOKEN", tc.token)
			set("CF_ACCOUNT_ID", tc.account)
			defer unset("CF_API_TOKEN")
			defer unset("CF_ACCOUNT_ID")

			var field api.Auth
			ppmock := pp.NewMock()
			ok := config.ReadAuth(ppmock, &field)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, &api.CloudflareAuth{Token: tc.token, AccountID: tc.account, BaseURL: ""}, field)
			} else {
				require.Nil(t, field)
			}
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

func useMemFS(memfs fstest.MapFS) {
	file.FS = memfs
}

func useDirFS() {
	file.FS = os.DirFS("/")
}

//nolint:funlen,paralleltest // environment vars and file system are global
func TestReadAuthTokenWithFile(t *testing.T) {
	unset("CF_API_TOKEN")
	unset("CF_API_TOKEN_FILE")
	unset("CF_ACCOUNT_ID")

	for name, tc := range map[string]struct {
		token         string
		tokenFile     string
		account       string
		actualPath    string
		actualContent string
		expected      string
		ok            bool
		ppRecords     []pp.Record
	}{
		"ok": {"", "test.txt", "secret account", "test.txt", "hello", "hello", true, nil},
		"both": {
			"123456789", "test.txt", "secret account", "test.txt", "hello", "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set`),
			},
		},
		"wrong.path": {
			"", "test.txt", "secret account", "wrong.txt", "hello", "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to read "test.txt": open test.txt: file does not exist`),
			},
		},
		"empty": {
			"", "test.txt", "secret account", "test.txt", "", "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `The token in the file specified by CF_API_TOKEN_FILE is empty`),
			},
		},
		"invalid path": {
			"", "dir", "secret account", "dir/test.txt", "hello", "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to read "dir": read dir: invalid argument`),
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("CF_API_TOKEN", tc.token)
			set("CF_API_TOKEN_FILE", tc.tokenFile)
			set("CF_ACCOUNT_ID", tc.account)
			defer unset("CF_API_TOKEN")
			defer unset("CF_API_TOKEN_FILE")
			defer unset("CF_ACCOUNT_ID")

			useMemFS(fstest.MapFS{
				tc.actualPath: &fstest.MapFile{
					Data:    []byte(tc.actualContent),
					Mode:    0o644,
					ModTime: time.Unix(1234, 5678),
					Sys:     nil,
				},
			})
			defer useDirFS()

			var field api.Auth
			ppmock := pp.NewMock()
			ok := config.ReadAuth(ppmock, &field)
			require.Equal(t, tc.ok, ok)
			if tc.expected != "" {
				require.Equal(t, &api.CloudflareAuth{Token: tc.expected, AccountID: tc.account, BaseURL: ""}, field)
			} else {
				require.Nil(t, field)
			}
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:paralleltest // environment vars are global
func TestReadDomainMap(t *testing.T) {
	unset("DOMAINS")
	unset("IP4_DOMAINS")
	unset("IP6_DOMAINS")

	for name, tc := range map[string]struct {
		domains    string
		ip4Domains string
		ip6Domains string
		expected   map[ipnet.Type][]api.FQDN
		ok         bool
		ppRecords  []pp.Record
	}{
		"full": {
			"  a1, a2", "b1,  b2,b2", "c1,c2",
			map[ipnet.Type][]api.FQDN{
				ipnet.IP4: {"a1", "a2", "b1", "b2"},
				ipnet.IP6: {"a1", "a2", "c1", "c2"},
			},
			true,
			nil,
		},
		"empty": {
			" ", "   ", "",
			map[ipnet.Type][]api.FQDN{
				ipnet.IP4: {},
				ipnet.IP6: {},
			},
			true,
			nil,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("DOMAINS", tc.domains)
			set("IP4_DOMAINS", tc.ip4Domains)
			set("IP6_DOMAINS", tc.ip6Domains)
			defer unset("DOMAINS")
			defer unset("IP4_DOMAINS")
			defer unset("IP6_DOMAINS")

			field := map[ipnet.Type][]api.FQDN{}
			ppmock := pp.NewMock()
			ok := config.ReadDomainMap(ppmock, field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}

//nolint:funlen,paralleltest // environment vars are global
func TestReadPolicyMap(t *testing.T) {
	unset("IP4_POLICY")
	unset("IP6_POLICY")

	var (
		cloudflare = detector.NewCloudflare()
		local      = detector.NewLocal()
		unmanaged  = detector.NewUnmanaged()
		ipify      = detector.NewIpify()
	)

	for name, tc := range map[string]struct {
		ip4Policy string
		ip6Policy string
		expected  map[ipnet.Type]detector.Policy
		ok        bool
		ppRecords []pp.Record
	}{
		"full": {
			"cloudflare", "ipify",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: cloudflare,
				ipnet.IP6: ipify,
			},
			true,
			nil,
		},
		"4": {
			"local", "  ",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: local,
				ipnet.IP6: local,
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default IP6_POLICY=local`),
			},
		},
		"6": {
			"    ", "ipify",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: unmanaged,
				ipnet.IP6: ipify,
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default IP4_POLICY=unmanaged`),
			},
		},
		"empty": {
			" ", "   ",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: unmanaged,
				ipnet.IP6: local,
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default IP4_POLICY=unmanaged`),
				pp.NewRecord(0, pp.Info, pp.EmojiBullet, `Use default IP6_POLICY=local`),
			},
		},
		"illformed": {
			" flare", "   ",
			map[ipnet.Type]detector.Policy{
				ipnet.IP4: unmanaged,
				ipnet.IP6: local,
			},
			false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to parse "flare": not a valid policy`),
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			set("IP4_POLICY", tc.ip4Policy)
			set("IP6_POLICY", tc.ip6Policy)
			defer unset("IP4_POLICY")
			defer unset("IP6_POLICY")

			field := map[ipnet.Type]detector.Policy{ipnet.IP4: unmanaged, ipnet.IP6: local}
			ppmock := pp.NewMock()
			ok := config.ReadPolicyMap(ppmock, field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, field)
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}
