package provider_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

func sleepCtx(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func TestHappyEyeballs(t *testing.T) {
	t.Parallel()

	ipNet := ipnet.IP4
	forever := time.Hour
	ip1111 := netip.MustParseAddr("1.1.1.1")
	ip1001 := netip.MustParseAddr("1.0.0.1")

	for name, tc := range map[string]struct {
		ip           netip.Addr
		method       provider.Method
		ok           bool
		timeout      time.Duration
		elapsed      time.Duration
		prepareMocks func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider)
	}{
		"no-alternative": {
			ip1111, provider.MethodPrimary, true,
			3 * time.Second, 0,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(false)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.1.1.1!")
						return ip1111, true
					}).Times(2)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
				p.EXPECT().HasAlternative(ipNet).Return(false)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
			},
		},
		"primary-instant": {
			ip1111, provider.MethodPrimary, true,
			3 * time.Second, 0,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.1.1.1!")
						return ip1111, true
					}).Times(2)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
				p.EXPECT().HasAlternative(ipNet).Return(true)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
			},
		},
		"primary-delayed/alternative-instant": {
			ip1001, provider.MethodAlternative, true,
			3 * time.Second, provider.HappyEyeballsAlternativeDelay,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.1.1.1!")
						sleepCtx(ctx, time.Second)
						return ip1111, true
					}).Times(1)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.0.0.1!")
						return ip1001, true
					}).Times(2)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.0.0.1!")
				ppfmt.EXPECT().Infof(pp.EmojiNow, "The server 1.0.0.1 responded before 1.1.1.1 does and will be used from now on")
				p.EXPECT().HasAlternative(ipNet).Return(true)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.0.0.1!")
			},
		},
		"primary-delayed/alternative-instant-fails": {
			ip1111, provider.MethodPrimary, true,
			3 * time.Second, time.Second,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.1.1.1!")
						sleepCtx(ctx, time.Second)
						return ip1111, true
					}).Times(2)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.0.0.1")
						return netip.Addr{}, false
					}).Times(1)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
				p.EXPECT().HasAlternative(ipNet).Return(true)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
			},
		},
		"primary-delayed/alternative-timeout": {
			ip1111, provider.MethodPrimary, true,
			3 * time.Second, time.Second,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.1.1.1!")
						sleepCtx(ctx, time.Second)
						return ip1111, true
					}).Times(2)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.0.0.1!")
						sleepCtx(ctx, forever)
						return ip1001, true
					}).Times(1)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
				p.EXPECT().HasAlternative(ipNet).Return(true)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.1.1.1!")
			},
		},
		"primary-instant-fails/alternative-instant": {
			ip1001, provider.MethodAlternative, true,
			3 * time.Second, 0,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.1.1.1")
						return netip.Addr{}, false
					}).Times(1)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.0.0.1!")
						return ip1001, true
					}).Times(2)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.0.0.1!")
				ppfmt.EXPECT().Infof(pp.EmojiNow, "The server 1.0.0.1 responded before 1.1.1.1 does and will be used from now on")
				p.EXPECT().HasAlternative(ipNet).Return(true)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.0.0.1!")
			},
		},
		"primary-instant-fails/alternative-instant-fails": {
			netip.Addr{},
			provider.MethodUnspecified, false,
			3 * time.Second, 0,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.1.1.1")
						return netip.Addr{}, false
					}).Times(1)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(_ context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.0.0.1")
						return netip.Addr{}, false
					}).Times(1)
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Can't get 1.1.1.1")
			},
		},
		"primary-timeout/alternative-delayed": {
			ip1001, provider.MethodAlternative, true,
			3 * time.Second, provider.HappyEyeballsAlternativeDelay + time.Second,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.1.1.1")
						sleepCtx(ctx, forever)
						return netip.Addr{}, false
					}).Times(1)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Infof(pp.EmojiGood, "Got 1.0.0.1!")
						sleepCtx(ctx, time.Second)
						return ip1001, true
					}).Times(2)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.0.0.1!")
				ppfmt.EXPECT().Infof(pp.EmojiNow, "The server 1.0.0.1 responded before 1.1.1.1 does and will be used from now on")
				p.EXPECT().HasAlternative(ipNet).Return(true)
				ppfmt.EXPECT().Infof(pp.EmojiGood, "Got 1.0.0.1!")
			},
		},
		"primary-timeout/alternative-timeout": {
			netip.Addr{},
			provider.MethodUnspecified, false,
			3 * time.Second, 3 * time.Second,
			func(ppfmt *mocks.MockPP, p *mocks.MockSplitProvider) {
				p.EXPECT().HasAlternative(ipNet).Return(true)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodPrimary).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.1.1.1")
						sleepCtx(ctx, forever)
						return netip.Addr{}, false
					}).Times(1)
				p.EXPECT().GetIP(gomock.Any(), gomock.Any(), ipNet, provider.MethodAlternative).DoAndReturn(
					func(ctx context.Context, ppfmt pp.PP, _ ipnet.Type, _ provider.Method) (netip.Addr, bool) {
						ppfmt.Noticef(pp.EmojiError, "Can't get 1.0.0.1")
						sleepCtx(ctx, forever)
						return netip.Addr{}, false
					}).Times(1)
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Can't get 1.1.1.1")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mockSP := mocks.NewMockSplitProvider(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP, mockSP)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			startTime := time.Now()
			p := provider.NewHappyEyeballs(mockSP)
			ip, method, ok := p.GetIP(ctx, mockPP, ipNet)
			require.Equal(t, tc.ip, ip)
			require.Equal(t, tc.method, method)
			require.Equal(t, tc.ok, ok)
			require.WithinDuration(t, startTime.Add(tc.elapsed), time.Now(), 100*time.Millisecond)

			if tc.ok {
				ip, method, ok = p.GetIP(ctx, mockPP, ipNet)
				require.Equal(t, tc.ip, ip)
				require.Equal(t, tc.method, method)
				require.Equal(t, tc.ok, ok)
			}
		})
	}
}
