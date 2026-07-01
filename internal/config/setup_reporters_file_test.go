package config_test

// vim: nowrap

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func shoutrrrFS(t *testing.T, path, content string) {
	t.Helper()
	file.SetFSForTesting(fstest.MapFS{
		path: &fstest.MapFile{
			Data:    []byte(content),
			Mode:    0o644,
			ModTime: time.Unix(1234, 5678),
			Sys:     nil,
		},
	})
	t.Cleanup(file.ResetFSForTesting)
}

//nolint:paralleltest,exhaustruct // environment vars and file.FS are global; table cases intentionally omit unused fields
func TestSetupReportersShoutrrrFile(t *testing.T) {
	const url1 = "generic+https://example.com/api/v1/postStuff"
	const url2 = "pushover://shoutrrr:token@userKey"

	for name, tc := range map[string]struct {
		shoutrrr      string // SHOUTRRR env value; "" means unset
		fileContent   string // file body; "" with useFile=false means no SHOUTRRR_FILE
		useFile       bool   // whether SHOUTRRR_FILE is set (to /shoutrrr on the mem FS)
		badPath       bool   // set SHOUTRRR_FILE to a non-absolute path
		missingFile   bool   // set SHOUTRRR_FILE to an absolute path absent from the (empty) mem FS
		ok            bool
		descriptions  []string // expected notifier services when ok and non-empty
		prepareMockPP func(*mocks.MockPP)
	}{
		"file only": {
			fileContent:  url1 + "\n" + url2,
			useFile:      true,
			ok:           true,
			descriptions: []string{"Generic", "Pushover"},
		},
		"env only": {
			shoutrrr:     url1,
			ok:           true,
			descriptions: []string{"Generic"},
		},
		"both equal": {
			shoutrrr:     url1 + "\n" + url2,
			fileContent:  url1 + "\n" + url2,
			useFile:      true,
			ok:           true,
			descriptions: []string{"Generic", "Pushover"},
		},
		"both equal reordered": {
			shoutrrr:     url1 + "\n" + url2,
			fileContent:  url2 + "\n" + url1,
			useFile:      true,
			ok:           true,
			descriptions: []string{"Generic", "Pushover"},
		},
		"both differ": {
			shoutrrr:    url1,
			fileContent: url2,
			useFile:     true,
			ok:          false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The URLs in SHOUTRRR and the file specified by SHOUTRRR_FILE differ; they must specify the same URLs")
			},
		},
		"both differ by multiplicity": {
			shoutrrr:    url1 + "\n" + url1 + "\n" + url2,
			fileContent: url1 + "\n" + url2,
			useFile:     true,
			ok:          false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The URLs in SHOUTRRR and the file specified by SHOUTRRR_FILE differ; they must specify the same URLs")
			},
		},
		"comment-only env plus populated file": {
			shoutrrr:     "# nothing here",
			fileContent:  url1,
			useFile:      true,
			ok:           true,
			descriptions: []string{"Generic"},
		},
		"empty file with env": {
			shoutrrr:     url1,
			fileContent:  "\n#comment\n",
			useFile:      true,
			ok:           true,
			descriptions: []string{"Generic"},
		},
		"space fail in file": {
			fileContent: url1 + " " + url2,
			useFile:     true,
			ok:          false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"Line %d of %s contains spaces, which suggests that multiple URLs were folded onto one line",
					1, "the file specified by SHOUTRRR_FILE")
				m.EXPECT().Infof(
					pp.EmojiHint,
					`If you meant multiple URLs, put each URL on its own line; if this is one URL, encode spaces as "%%20"`)
				m.EXPECT().Infof(
					pp.EmojiHint,
					`If you use YAML folded block style ">", switch to literal block style "|"`)
			},
		},
		// Regression guard for the raw-read invariant: leading blank lines in the
		// file must NOT shift diagnostic line numbers. This fails if the file
		// source is ever routed through file.ReadString (which trims leading
		// blank lines) instead of file.ReadRawString. The folded URL sits on the
		// third line, so the diagnostic must say line 3, not line 1.
		"space fail in file after leading blank lines": {
			fileContent: "\n\n" + url1 + " " + url2,
			useFile:     true,
			ok:          false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"Line %d of %s contains spaces, which suggests that multiple URLs were folded onto one line",
					3, "the file specified by SHOUTRRR_FILE")
				m.EXPECT().Infof(
					pp.EmojiHint,
					`If you meant multiple URLs, put each URL on its own line; if this is one URL, encode spaces as "%%20"`)
				m.EXPECT().Infof(
					pp.EmojiHint,
					`If you use YAML folded block style ">", switch to literal block style "|"`)
			},
		},
		"non-absolute path": {
			badPath: true,
			ok:      false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"The path %s is not absolute; to use an absolute path, prefix it with /",
					gomock.Any())
			},
		},
		// A configured but unreadable file is an error, distinct from an unset
		// SHOUTRRR_FILE. This proves SetupReporters propagates ReadRawString
		// failure instead of silently treating the file source as empty.
		"missing file": {
			missingFile: true,
			ok:          false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(
					pp.EmojiUserError,
					"Failed to read %s: %v",
					"/missing", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			unset(t, "HEALTHCHECKS", "UPTIMEKUMA", "SHOUTRRR", "SHOUTRRR_FILE")
			if tc.shoutrrr != "" {
				store(t, "SHOUTRRR", tc.shoutrrr)
			}
			switch {
			case tc.badPath:
				store(t, "SHOUTRRR_FILE", "relative/path")
			case tc.missingFile:
				file.SetFSForTesting(fstest.MapFS{})
				t.Cleanup(file.ResetFSForTesting)
				store(t, "SHOUTRRR_FILE", "/missing")
			case tc.useFile:
				shoutrrrFS(t, "shoutrrr", tc.fileContent)
				store(t, "SHOUTRRR_FILE", "/shoutrrr")
			}

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			_, nt, ok := config.SetupReporters(mockPP)
			require.Equal(t, tc.ok, ok)

			if tc.ok && len(tc.descriptions) > 0 {
				ns, isComposed := nt.(notifier.Composed)
				require.True(t, isComposed)
				require.Len(t, ns, 1)
				s, isShoutrrr := ns[0].(notifier.Shoutrrr)
				require.True(t, isShoutrrr)
				require.Equal(t, tc.descriptions, s.ServiceDescriptions)
			}
		})
	}
}
