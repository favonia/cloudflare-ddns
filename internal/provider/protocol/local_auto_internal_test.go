package protocol

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var errDialFailed = errors.New("dial failed")

type stubConn struct {
	localAddr net.Addr
	closeErr  error
}

func (c stubConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c stubConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c stubConn) Close() error                     { return c.closeErr }
func (c stubConn) LocalAddr() net.Addr              { return c.localAddr }
func (c stubConn) RemoteAddr() net.Addr             { return &net.UDPAddr{IP: nil, Port: 0, Zone: ""} }
func (c stubConn) SetDeadline(time.Time) error      { return nil }
func (c stubConn) SetReadDeadline(time.Time) error  { return nil }
func (c stubConn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr struct{}

func (dummyAddr) Network() string { return "dummy" }
func (dummyAddr) String() string  { return "dummy/string" }

type noticeCall struct {
	emoji  pp.Emoji
	format string
	args   []any
}

type stubPP struct {
	noticeCalls []noticeCall
}

func (*stubPP) IsShowing(pp.Verbosity) bool    { return true }
func (p *stubPP) Indent() pp.PP                { return p }
func (*stubPP) BlankLineIfVerbose()            {}
func (*stubPP) Infof(pp.Emoji, string, ...any) {}
func (p *stubPP) Noticef(emoji pp.Emoji, format string, args ...any) {
	p.noticeCalls = append(p.noticeCalls, noticeCall{emoji: emoji, format: format, args: args})
}
func (*stubPP) Suppress(pp.ID)                              {}
func (*stubPP) InfoOncef(pp.ID, pp.Emoji, string, ...any)   {}
func (*stubPP) NoticeOncef(pp.ID, pp.Emoji, string, ...any) {}

func TestLocalAutoGetIPsWithDialContextSuccess(t *testing.T) {
	t.Parallel()

	mockPP := &stubPP{noticeCalls: nil}
	provider := LocalAuto{
		ProviderName:  "auto",
		RemoteUDPAddr: "198.51.100.10:53",
	}
	expected := netip.MustParseAddr("10.0.0.8")

	targets := provider.getIPsWithDialContext(
		context.Background(),
		mockPP,
		ipnet.IP4,
		func(_ context.Context, network, remoteUDPAddr string) (net.Conn, error) {
			require.Equal(t, ipnet.IP4.UDPNetwork(), network)
			require.Equal(t, provider.RemoteUDPAddr, remoteUDPAddr)
			return stubConn{
				localAddr: &net.UDPAddr{IP: expected.AsSlice(), Port: 12345, Zone: ""},
				closeErr:  nil,
			}, nil
		},
	)

	require.True(t, targets.Available)
	require.Equal(t, []netip.Addr{expected}, targets.IPs)
}

func TestLocalAutoGetIPsWithDialContextInvalidLocalAddr(t *testing.T) {
	t.Parallel()

	mockPP := &stubPP{noticeCalls: nil}
	provider := LocalAuto{
		ProviderName:  "auto",
		RemoteUDPAddr: "198.51.100.10:53",
	}

	targets := provider.getIPsWithDialContext(
		context.Background(),
		mockPP,
		ipnet.IP4,
		func(context.Context, string, string) (net.Conn, error) {
			return stubConn{localAddr: dummyAddr{}, closeErr: nil}, nil
		},
	)

	require.False(t, targets.Available)
	require.Nil(t, targets.IPs)
	require.Len(t, mockPP.noticeCalls, 1)
	require.Equal(t, pp.EmojiImpossible, mockPP.noticeCalls[0].emoji)
	require.Equal(t, "Unexpected UDP source address data %q of type %T", mockPP.noticeCalls[0].format)
	require.Equal(t, []any{"dummy/string", dummyAddr{}}, mockPP.noticeCalls[0].args)
}

func TestLocalAutoGetIPsWithDialContextDialFailure(t *testing.T) {
	t.Parallel()

	mockPP := &stubPP{noticeCalls: nil}
	provider := LocalAuto{
		ProviderName:  "auto",
		RemoteUDPAddr: "198.51.100.10:53",
	}

	targets := provider.getIPsWithDialContext(
		context.Background(),
		mockPP,
		ipnet.IP6,
		func(context.Context, string, string) (net.Conn, error) {
			return nil, errDialFailed
		},
	)

	require.False(t, targets.Available)
	require.Nil(t, targets.IPs)
	require.Len(t, mockPP.noticeCalls, 1)
	require.Equal(t, pp.EmojiError, mockPP.noticeCalls[0].emoji)
	require.Equal(t, "Failed to detect a local %s address: %v", mockPP.noticeCalls[0].format)
	require.Len(t, mockPP.noticeCalls[0].args, 2)
	require.Equal(t, "IPv6", mockPP.noticeCalls[0].args[0])
	detectedErr, ok := mockPP.noticeCalls[0].args[1].(error)
	require.True(t, ok)
	require.ErrorIs(t, detectedErr, errDialFailed)
}
