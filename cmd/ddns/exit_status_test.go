package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestOperationSuccess(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"update", "cleanup", "stop"} {
		for _, tc := range []struct {
			name           string
			response       setter.ResponseCode
			reporterOK     bool
			cancelOnReport bool
			cancelInWork   bool
			timeout        bool
			want           bool
		}{
			{"noop", setter.ResponseNoop, true, false, false, false, true},
			{"updated", setter.ResponseUpdated, true, false, false, false, true},
			{"failed", setter.ResponseFailed, true, false, false, false, false},
			{"async", setter.ResponseUpdating, true, false, false, false, true},
			{"report-failed", setter.ResponseUpdated, false, false, false, false, true},
			{"late-cancel", setter.ResponseUpdated, false, true, false, false, true},
			{"failure-preserved", setter.ResponseFailed, false, true, false, false, false},
			{"interrupted", setter.ResponseFailed, true, false, true, false, false},
			{"timeout", setter.ResponseFailed, true, false, false, true, false},
		} {
			if mode == "cleanup" && tc.response == setter.ResponseUpdating {
				continue // CleanupWait does not return an in-progress result.
			}
			if mode == "stop" && (tc.cancelInWork || tc.cancelOnReport) {
				continue // Scheduled cleanup uses a fresh, uncanceled shutdown context.
			}
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				ctrl := gomock.NewController(t)
				s := mocks.NewMockSetter(ctrl)
				hb := mocks.NewMockHeartbeat(ctrl)
				nt := mocks.NewMockNotifier(ctrl)
				ppfmt := pp.NewSilent()
				workCtx, cancel := context.WithCancel(t.Context())
				defer cancel()
				reportCtx := t.Context()
				list := api.WAFList{AccountID: "account", Name: "list"}
				cfg := &config.UpdateConfig{
					Provider: map[ipnet.Family]provider.Provider{ipnet.IP4: provider.NewStaticEmpty()},
					Domains:  nil, HostID6: nil, WAFLists: []api.WAFList{list},
					DefaultPrefixLen: map[ipnet.Family]int{ipnet.IP4: 32}, DetectionFilter: nil,
					TTL: api.TTLAuto, Proxied: nil, RecordComment: "", WAFListDescription: "", WAFListItemComment: "",
					DetectionTimeout: time.Second, UpdateTimeout: time.Second,
				}
				if tc.timeout {
					cfg.UpdateTimeout = time.Millisecond
				}
				respond := func(ctx context.Context) setter.ResponseCode {
					if tc.cancelInWork {
						cancel()
						require.ErrorIs(t, ctx.Err(), context.Canceled)
					}
					if tc.timeout {
						<-ctx.Done()
						require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
					}
					return tc.response
				}
				if mode == "update" {
					s.EXPECT().SetWAFList(gomock.Any(), ppfmt, list, "", gomock.Any(), "").DoAndReturn(
						func(ctx context.Context, _ pp.PP, _ api.WAFList, _ string, _ map[ipnet.Family]setter.WAFTargets, _ string) setter.ResponseCode {
							return respond(ctx)
						})
				} else {
					cleanupMode := api.CleanupWait
					if mode == "stop" {
						cleanupMode = api.CleanupAllowAsync
					}
					s.EXPECT().FinalClearWAFList(gomock.Any(), ppfmt, list, "", gomock.Any(), cleanupMode).DoAndReturn(
						func(ctx context.Context, _ pp.PP, _ api.WAFList, _ string, _ map[ipnet.Family]bool, _ api.CleanupMode) setter.ResponseCode {
							return respond(ctx)
						})
				}
				report := func(ctx context.Context, _ pp.PP, msg heartbeat.Message) bool {
					if tc.cancelOnReport {
						cancel()
					}
					require.NoError(t, ctx.Err())
					require.Equal(t, tc.want, msg.OK)
					return tc.reporterOK
				}
				if mode == "stop" {
					hb.EXPECT().Log(reportCtx, ppfmt, gomock.Any()).DoAndReturn(report)
				} else {
					hb.EXPECT().Ping(reportCtx, ppfmt, gomock.Any()).DoAndReturn(report)
				}
				nt.EXPECT().Send(reportCtx, ppfmt, gomock.Any()).DoAndReturn(func(ctx context.Context, _ pp.PP, _ notifier.Notification) bool {
					require.NoError(t, ctx.Err())
					return tc.reporterOK
				})
				var ok bool
				switch mode {
				case "update":
					ok = runOnceUpdate(workCtx, reportCtx, ppfmt, cfg, hb, nt, s)
				case "cleanup":
					ok = runOnceCleanup(workCtx, reportCtx, ppfmt, cfg, hb, nt, s)
				case "stop":
					ok = stopUpdating(reportCtx, ppfmt, &config.LifecycleConfig{UpdateCron: nil, UpdateOnStart: false, DeleteOnStop: true}, cfg, hb, nt, s)
				}
				require.Equal(t, tc.want, ok)
			})
		}
	}
}
